package httpw

import (
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func pingServer(addr string) *Server {
	return NewServer(Options{
		Addr: addr,
		Routes: []Route{{
			Method:  http.MethodGet,
			Pattern: "/ping",
			Handler: func(c *gin.Context) { c.String(http.StatusOK, "pong") },
		}},
	})
}

func TestListenReportsAConflictBeforeServe(t *testing.T) {
	first := pingServer("127.0.0.1:0")
	if err := first.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer first.Stop()

	second := pingServer(first.Addr().String())
	if err := second.Listen(); err == nil {
		second.Stop()
		t.Fatal("Listen on a bound port succeeded; want the bind error")
	}
}

func TestServeAcceptsOnTheBoundListener(t *testing.T) {
	s := pingServer("127.0.0.1:0")
	if err := s.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer s.Stop()

	//a route mounted between bind and serve is served
	s.AddRoutes("late", []Route{{
		Method:  http.MethodGet,
		Pattern: "/ping",
		Handler: func(c *gin.Context) { c.String(http.StatusOK, "late") },
	}})
	if err := s.Serve(); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	cli := NewClient(nil, nil, "")
	for path, want := range map[string]string{"/ping": "pong", "/late/ping": "late"} {
		rsp, body, err := cli.Send(http.MethodGet, s.Addr().String()+path, nil)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if rsp.StatusCode != http.StatusOK || string(body) != want {
			t.Fatalf("GET %s = %d %q, want 200 %q", path, rsp.StatusCode, body, want)
		}
	}
}

func TestStopClosesAListenerNeverServed(t *testing.T) {
	s := pingServer("127.0.0.1:0")
	if err := s.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := s.Addr().String()
	s.Stop()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port still held after Stop: %v", err)
	}
	ln.Close()
}

func TestServeOrderingErrors(t *testing.T) {
	s := pingServer("127.0.0.1:0")
	if err := s.Serve(); !errors.Is(err, ErrNotListening) {
		t.Fatalf("Serve before Listen = %v, want ErrNotListening", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Listen(); err != nil {
		t.Fatalf("Listen on a bound server = %v, want nil", err)
	}
	if err := s.Serve(); !errors.Is(err, ErrAlreadyServing) {
		t.Fatalf("second Serve = %v, want ErrAlreadyServing", err)
	}
	s.Stop()
	if err := s.Start(); !errors.Is(err, ErrServerStopped) {
		t.Fatalf("Start after Stop = %v, want ErrServerStopped", err)
	}
}

func TestStopWithoutListen(t *testing.T) {
	pingServer("127.0.0.1:0").Stop()
}
