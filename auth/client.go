package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Option func(*http.Transport) error

func WithInsecure(t *http.Transport) error {
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return nil
}

func WithSSHTunnel(proxy string, config *ssh.ClientConfig) (Option, error) {
	// parse a user@host:port string
	user, host, port, err := parseSSHTarget(proxy)
	if err != nil {
		return nil, fmt.Errorf("unable to parse ssh proxy %s: %w", proxy, err)
	}

	// if a user was provided in the proxy string, override the config user
	if user != "" {
		config.User = user
	}

	// default port if not provided
	if port == "" {
		port = "22"
	}

	// we use the current transport's DialContext and wraps it in an ssh client's DialContext
	return func(t *http.Transport) error {
		hostPort := net.JoinHostPort(host, port)
		conn, err := t.DialContext(context.Background(), "tcp", hostPort)
		if err != nil {
			return fmt.Errorf("unable to dial ssh proxy %s: %w", proxy, err)
		}

		c, chans, reqs, err := ssh.NewClientConn(conn, hostPort, config)
		if err != nil {
			return fmt.Errorf("unable to create ssh client conn: %w", err)
		}

		t.DialContext = ssh.NewClient(c, chans, reqs).DialContext
		return nil
	}, nil
}

func Client(host, user, pass string, opts ...Option) (*http.Client, error) {
	// this is the golang 1.25 http.DefaultTransport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		err := opt(transport)
		if err != nil {
			return nil, fmt.Errorf("unable to apply option %T: %w", opt, err)
		}
	}

	c := &http.Client{Transport: transport}

	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/auth/login",
	}

	authRequestDoc := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: user,
		Password: pass,
	}

	requestBody, err := json.Marshal(authRequestDoc)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal auth request: %w", err)
	}

	authRequest, err := http.NewRequest("POST", u.String(), bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("unable to create http request: %w", err)
	}

	authRequest.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(authRequest)
	if err != nil {
		return nil, fmt.Errorf("could not http request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusForbidden:
		return nil, ErrInvalidCredentials
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	// create a new Transport, using the old transport from c
	t := &authTransport{
		RoundTripper: c.Transport,
	}

	// replace c's transport with the auth transport
	c.Transport = t

	jwt := resp.Header.Get("Authorization")
	t.token.Store(&jwt)

	for _, v := range resp.Cookies() {
		if v.Name == "refresh_token" {
			t.refreshToken = v.Value

			// dont look further
			break
		}
	}

	// if we have a refreshToken, keep our JWT alive
	if t.refreshToken != "" {
		// start loop that refreshes token
		go t.keepalive(u)
	}

	return c, nil
}

type authTransport struct {
	http.RoundTripper
	token           atomic.Pointer[string]
	refreshToken    string
	refreshTokenErr atomic.Pointer[error]
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request to avoid modifying the original one
	clonedReq := req.Clone(req.Context())

	t := a.token.Load()
	clonedReq.Header.Set("Authorization", *t)

	resp, err := a.RoundTripper.RoundTrip(clonedReq)
	if err != nil {
		refreshErr := a.refreshTokenErr.Load()
		if refreshErr != nil {
			return resp, errors.Join(err, fmt.Errorf("before that: %w", *refreshErr))
		}

		return resp, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		refreshErr := a.refreshTokenErr.Load()
		if refreshErr != nil {
			return resp, errors.Join(err, fmt.Errorf("status 401 unauthorized, prior to that: %w", *refreshErr))
		}
	}

	return resp, err
}

func (a *authTransport) keepalive(url url.URL) {
	url.Path = "/auth/refresh"
	for {
		// TODO: use actual expire value from JWT
		// 	for now, we know its usually 10 minuttes, so we refresh at every 9 minuttes
		//
		// TODO: we should learn if we are even supposed to refresh the token before
		//   the jwt expires - to me it seems like the refresh_token works even though
		//	 hours might have gone by - this could significantly simplify the code as
		//   we wont need any routines to do stuff in the background
		//
		// TODO: give consumers a way to cancel this routine
		// 	In the context of iectl, it think its okay that this jwt is refreshed forever
		time.Sleep(time.Minute * 9)
		req, err := http.NewRequest("GET", url.String(), nil)
		if err != nil {
			err = fmt.Errorf("refresh token: could not create http request: %w", err)
			a.refreshTokenErr.Store(&err)
			break
		}

		c := &http.Cookie{
			Name:  "refresh_token",
			Value: a.refreshToken,
		}

		req.AddCookie(c)

		httpClient := &http.Client{Transport: a}
		resp, err := httpClient.Do(req)
		if err != nil {
			err = fmt.Errorf("refresh token: request failed: %w", err)
			a.refreshTokenErr.Store(&err)
			break
		}

		// the body have nothing of interest
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("refresh token: request returned unexpected status code: %d", resp.StatusCode)
			a.refreshTokenErr.Store(&err)
			break
		}

		jwt := resp.Header.Get("Authorization")
		if jwt == "" {
			err = fmt.Errorf("refresh token: a successive call to refresh did not include a new JWT in its response")
			a.refreshTokenErr.Store(&err)
		}

		a.token.Store(&jwt)
	}
}

func parseSSHTarget(s string) (username, hostname, port string, err error) {
	// Split username from host:port
	var hostPort string
	if idx := strings.Index(s, "@"); idx != -1 {
		username = s[:idx]
		hostPort = s[idx+1:]
	} else {
		hostPort = s
	}

	// Split hostname from port
	hostname, port, err = net.SplitHostPort(hostPort)
	if err != nil {
		// If there's no port, SplitHostPort will fail
		// In that case, the whole thing is just the hostname
		hostname = hostPort
		port = ""
		err = nil
	}

	return
}
