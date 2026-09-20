package httpw

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// DefaultTimeout bounds a request whose context carries no deadline: the
// longest it waits for its response, body included. A request that carries a
// deadline is bounded by that alone - two clocks on one request race, and the
// caller's deadline is the one that knows how long the answer is worth.
const DefaultTimeout = 10 * time.Second

// PingAfterIdle and PingTimeout are the HTTP/2 health check both ends of a
// connection run: one that has read nothing for PingAfterIdle is pinged, and
// closed if the ping is not answered within PingTimeout. A request its caller
// gave up on is cancelled at the server by a stream reset, which a partition
// never delivers; without the ping the server works on until TCP keepalive
// notices, after minutes.
const (
	PingAfterIdle = 5 * time.Second
	PingTimeout   = 5 * time.Second
)

type Client struct {
	cli    http.Client
	scheme string
	//timeout bounds a request that carries no deadline of its own
	timeout time.Duration
}

func NewClient(cert *tls.Certificate, caPool *x509.CertPool, serverName string) *Client {

	if cert != nil && caPool != nil {
		t := &http.Transport{
			MaxConnsPerHost:     64,
			MaxIdleConnsPerHost: 64,
			MaxIdleConns:        512,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
			DisableKeepAlives:   false,
		}

		t.TLSClientConfig = &tls.Config{
			Certificates:       []tls.Certificate{*cert},
			RootCAs:            caPool,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false, // DO NOT disable in production
			ServerName:         serverName,
		}
		//the HTTP/2 transport behind t, for its health check. net/http's own
		//HTTP2Config would do the same and needs Go 1.24. It errs only on a
		//transport already configured for HTTP/2, which a fresh one is not
		if t2, err := http2.ConfigureTransports(t); err == nil {
			t2.ReadIdleTimeout = PingAfterIdle
			t2.PingTimeout = PingTimeout
		}
		return &Client{
			scheme:  "https",
			cli:     http.Client{Transport: t},
			timeout: DefaultTimeout,
		}
	} else {
		t := &http2.Transport{
			AllowHTTP: true, // allow h2c
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
			ReadIdleTimeout: PingAfterIdle,
			PingTimeout:     PingTimeout,
		}
		return &Client{
			cli:     http.Client{Transport: t},
			scheme:  "http",
			timeout: DefaultTimeout,
		}
	}
}

// SendRequest sends req and returns without reading the response body. Closing
// the body is the caller's, and it also ends the client's own bound when one
// applied.
func (w *Client) SendRequest(req *http.Request) (rsp *http.Response, err error) {
	//set the right scheme
	req.URL.Scheme = w.scheme
	req, release := w.bound(req)
	//send request
	if rsp, err = w.cli.Do(req); err != nil {
		release()
		err = fmt.Errorf("Send http request: %w", err)
		return
	}
	rsp.Body = releaseOnClose{ReadCloser: rsp.Body, release: release}
	return
}

// read request then read all response body
func (w *Client) Send(method string, url string, body io.Reader) (rsp *http.Response, rspBody []byte, err error) {
	var req *http.Request
	url = fmt.Sprintf("%s://%s", w.scheme, url)
	if req, err = http.NewRequest(method, url, body); err != nil {
		return nil, nil, fmt.Errorf("Create http request: %w", err)
	}
	req, release := w.bound(req)
	defer release()
	//send request
	if rsp, err = w.cli.Do(req); err != nil {
		err = fmt.Errorf("Send http request: %w", err)
		return
	}

	//read response body binary
	if rsp.Body != nil {
		defer rsp.Body.Close()
		if rspBody, err = ioutil.ReadAll(rsp.Body); err != nil {
			err = fmt.Errorf("Read http response body: %w", err)
		}
	}
	return
}

// bound gives req the client's own bound when its context carries no deadline,
// and leaves it untouched when it does. The cancel function is always non-nil.
func (w *Client) bound(req *http.Request) (*http.Request, context.CancelFunc) {
	if _, ok := req.Context().Deadline(); ok || w.timeout <= 0 {
		return req, func() {}
	}
	ctx, cancel := context.WithTimeout(req.Context(), w.timeout)
	return req.WithContext(ctx), cancel
}

// releaseOnClose ends a request's bound when its body is closed: the caller reads
// the body after SendRequest returns, and ending the bound any earlier would cut
// that read short.
type releaseOnClose struct {
	io.ReadCloser
	release context.CancelFunc
}

func (b releaseOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.release()
	return err
}
