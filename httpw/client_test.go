package httpw

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
)

// slowServer answers "done" after delay, or stops when its caller goes away.
func slowServer(t *testing.T, delay time.Duration) string {
	t.Helper()
	s := NewServer(Options{
		Addr: "127.0.0.1:0",
		Routes: []Route{{
			Method:  http.MethodGet,
			Pattern: "/slow",
			Handler: func(c *gin.Context) {
				select {
				case <-time.After(delay):
				case <-c.Request.Context().Done():
				}
				c.String(http.StatusOK, "done")
			},
		}},
	})
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)
	return s.Addr().String()
}

// A request that carries a deadline is bounded by it alone: the client's own
// bound does not cut it short.
func TestADeadlineIsTheOnlyClock(t *testing.T) {
	addr := slowServer(t, 150*time.Millisecond)
	c := NewClient(nil, nil, "")
	c.timeout = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/slow", nil)
	rsp, err := c.SendRequest(req)
	if err != nil {
		t.Fatalf("a request within its own deadline was cut short: %v", err)
	}
	defer rsp.Body.Close()
	if body, _ := io.ReadAll(rsp.Body); string(body) != "done" {
		t.Fatalf("body %q", body)
	}
}

// A request that carries no deadline still ends: the client's own bound is
// what the control plane's requests, built with no context, rely on.
func TestARequestWithNoDeadlineIsStillBounded(t *testing.T) {
	addr := slowServer(t, 2*time.Second)
	c := NewClient(nil, nil, "")
	c.timeout = 50 * time.Millisecond

	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/slow", nil)
	start := time.Now()
	if rsp, err := c.SendRequest(req); err == nil {
		rsp.Body.Close()
		t.Fatal("a request with no deadline waited for a slow server")
	}
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("waited %s for a 50 ms bound", waited)
	}
}

// The bound lasts until the body is closed, so a body read after SendRequest
// returns is not cut short.
func TestTheBodyIsReadableAfterSendRequestReturns(t *testing.T) {
	addr := slowServer(t, 0)
	c := NewClient(nil, nil, "")

	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/slow", nil)
	rsp, err := c.SendRequest(req)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	defer rsp.Body.Close()
	if body, err := io.ReadAll(rsp.Body); err != nil || string(body) != "done" {
		t.Fatalf("body %q, %v", body, err)
	}

	_, body, err := c.Send(http.MethodGet, addr+"/slow", nil)
	if err != nil || string(body) != "done" {
		t.Fatalf("Send: body %q, %v", body, err)
	}
}

// Both ends run the HTTP/2 health check, so a connection lost to a partition is
// closed within seconds and the requests on it end.
func TestBothEndsPingAnIdleConnection(t *testing.T) {
	c := NewClient(nil, nil, "")
	t2, ok := c.cli.Transport.(*http2.Transport)
	if !ok {
		t.Fatalf("h2c client transport is %T", c.cli.Transport)
	}
	if t2.ReadIdleTimeout != PingAfterIdle || t2.PingTimeout != PingTimeout {
		t.Errorf("client pings after %s, times out after %s", t2.ReadIdleTimeout, t2.PingTimeout)
	}
	if c.cli.Timeout != 0 {
		t.Errorf("the client still runs a total timeout of %s on every request", c.cli.Timeout)
	}
	if s := http2Server(); s.ReadIdleTimeout != PingAfterIdle || s.PingTimeout != PingTimeout {
		t.Errorf("server pings after %s, times out after %s", s.ReadIdleTimeout, s.PingTimeout)
	}
}
