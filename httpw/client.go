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

	"github.com/reogac/utils"
	"golang.org/x/net/http2"
)

type Client struct {
	cli    http.Client
	scheme string
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
		return &Client{
			scheme: "https",
			cli: http.Client{
				Timeout:   5 * time.Second,
				Transport: t,
			},
		}
	} else {
		t := &http2.Transport{
			AllowHTTP: true, // allow h2c
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		}
		return &Client{
			cli: http.Client{
				Timeout:   5 * time.Second,
				Transport: t,
			},
			scheme: "http",
		}
	}
}

// send request but do not read response body
func (w *Client) SendRequest(req *http.Request) (rsp *http.Response, err error) {
	//set the right scheme
	req.URL.Scheme = w.scheme
	//send request
	if rsp, err = w.cli.Do(req); err != nil {
		err = utils.WrapError("Send http request", err)
		return
	}
	return
}

// read request then read all response body
func (w *Client) Send(method string, url string, body io.Reader) (rsp *http.Response, rspBody []byte, err error) {
	var req *http.Request
	url = fmt.Sprintf("%s://%s", w.scheme, url)
	if req, err = http.NewRequest(method, url, body); err != nil {
		return nil, nil, utils.WrapError("Create http request", err)
	}
	//send request
	if rsp, err = w.cli.Do(req); err != nil {
		err = utils.WrapError("Send http request", err)
		return
	}

	//read response body binary
	if rsp.Body != nil {
		defer rsp.Body.Close()
		if rspBody, err = ioutil.ReadAll(rsp.Body); err != nil {
			err = utils.WrapError("Read http response body", err)
		}
	}
	return
}
