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

type Option func(*http.Client) error

func WithInsecure(c *http.Client) error {
	t, ok := c.Transport.(*http.Transport)
	if !ok {
		return fmt.Errorf("transport is not *http.Transport")
	}
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
	return func(c *http.Client) error {
		t, ok := c.Transport.(*http.Transport)
		if !ok {
			return fmt.Errorf("transport is not *http.Transport")
		}

		hostPort := net.JoinHostPort(host, port)
		conn, err := t.DialContext(context.Background(), "tcp", hostPort)
		if err != nil {
			return fmt.Errorf("unable to dial ssh proxy %s: %w", proxy, err)
		}

		sshConn, chans, reqs, err := ssh.NewClientConn(conn, hostPort, config)
		if err != nil {
			return fmt.Errorf("unable to create ssh client conn: %w", err)
		}

		t.DialContext = ssh.NewClient(sshConn, chans, reqs).DialContext
		return nil
	}, nil
}

func WithCredentials(host, user, pass string) Option {
	return func(c *http.Client) error {
		t := &authTransport{
			RoundTripper: c.Transport,
		}
		c.Transport = t
		return t.login(host, user, pass)
	}
}

func Client(opts ...Option) (*http.Client, error) {
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

	c := &http.Client{Transport: transport}

	// Apply options
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, fmt.Errorf("unable to apply option: %w", err)
		}
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
	if t != nil {
		clonedReq.Header.Set("Authorization", *t)
	}

	// warning: we try to fetch the refreshToken loop's error
	// as a convenience to the caller - but it is racy - it might be stuck at something
	// or starved of resources hence, the refreshTokenErr might not be set yet - or it might
	// not have encountered an error at all...
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
			return resp, fmt.Errorf("status 401 unauthorized, prior to that: %w", *refreshErr)
		}
	}

	return resp, err
}

func (a *authTransport) login(host, user, pass string) error {
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
		return fmt.Errorf("unable to marshal auth request: %w", err)
	}

	authRequest, err := http.NewRequest("POST", u.String(), bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("unable to create http request: %w", err)
	}

	authRequest.Header.Set("Content-Type", "application/json")

	// Use the underlying RoundTripper to execute the request
	resp, err := a.RoundTripper.RoundTrip(authRequest)
	if err != nil {
		return fmt.Errorf("could not http request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusForbidden:
		return ErrInvalidCredentials
	case http.StatusOK:
	default:
		return fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	jwt := resp.Header.Get("Authorization")
	if jwt == "" {
		return fmt.Errorf("authorization header not found in response")
	}

	a.token.Store(&jwt)

	for _, v := range resp.Cookies() {
		if v.Name == "refresh_token" {
			a.refreshToken = v.Value

			// start keepalive routine
			go a.keepalive(u)
			break
		}
	}

	return nil
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
