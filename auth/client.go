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
	"sync"
	"time"
)

type timeoutConn struct {
	net.Conn
	writeTimeout time.Duration
}

func (c *timeoutConn) Write(b []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	return c.Conn.Write(b)
}

var ErrInvalidCredentials = errors.New("invalid credentials")

func Client(host, user, pass string, insecure bool) (*http.Client, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{Timeout: time.Minute}).DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return &timeoutConn{
				Conn:         conn,
				writeTimeout: time.Minute,
			}, nil
		},
	}

	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
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

	var refreshToken string
	cookies := resp.Cookies()
	for _, v := range cookies {
		if v.Name == "refresh_token" {
			refreshToken = v.Value
		}
	}

	t := &authTransport{
		RoundTripper: c.Transport,
		token:        resp.Header.Get("Authorization"),
		refreshToken: refreshToken,
	}

	// if the refreshToken is non empty, initialize a keepalive routine
	if refreshToken != "" {
		go t.keepalive(u)
	}

	c.Transport = t

	return c, nil
}

type authTransport struct {
	sync.RWMutex
	http.RoundTripper
	token           string
	refreshToken    string
	refreshTokenErr error
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request to avoid modifying the original one
	clonedReq := req.Clone(req.Context())

	a.RLock()
	clonedReq.Header.Set("Authorization", a.token)
	a.RUnlock()

	return a.RoundTripper.RoundTrip(clonedReq)
}

func (a *authTransport) keepalive(url url.URL) {
	url.Path = "/auth/refresh"
	for {
		// TODO: use actual expire value from JWT
		// for now, we know its usually 10 minuttes, so we refresh at every 9 minuttes
		time.Sleep(time.Minute * 9)
		req, err := http.NewRequest("GET", url.String(), nil)
		if err != nil {
			a.refreshTokenErr = fmt.Errorf("refresh token: could not create http request: %w", err)
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
			a.refreshTokenErr = fmt.Errorf("refresh token: request failed: %w", err)
			break
		}

		// the body have nothing of interest
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			a.refreshTokenErr = fmt.Errorf("refresh token: request returned unexpected status code: %d", resp.StatusCode)
			break
		}

		a.Lock()
		a.token = resp.Header.Get("Authorization")
		a.Unlock()
	}
}
