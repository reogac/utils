package httpw

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"sync"

	//	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	ErrNotListening   = errors.New("httpw: server is not listening")
	ErrAlreadyServing = errors.New("httpw: server is already serving")
	ErrServerStopped  = errors.New("httpw: server is stopped")
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type Route struct {
	Method  string
	Pattern string
	Handler func(*gin.Context)
}

type RouteGroup struct {
	Name   string
	Routes []Route
}

type Options struct {
	Addr        string
	Cert        tls.Certificate
	CaPool      *x509.CertPool
	Routes      []Route
	Middlewares []gin.HandlerFunc
}

type Server struct {
	Srv    *http.Server
	router gin.IRouter
	wg     sync.WaitGroup

	mu      sync.Mutex
	ln      net.Listener
	serving bool
	stopped bool
}

func NewServer(opts Options) *Server {

	router := gin.New()
	/*
		router.Use(cors.New(cors.Config{
			AllowMethods: []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
			AllowHeaders: []string{
				"Origin", "Content-Length", "Content-Type", "User-Agent", "Referrer", "Host",
				"Token", "X-Requested-With",
			},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			AllowAllOrigins:  true,
			MaxAge:           86400,
		}))
	*/
	for _, m := range opts.Middlewares {
		router.Use(m)
	}

	var srv *http.Server
	if opts.CaPool != nil {
		// Configure TLS with optional client certificate validation
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{opts.Cert},
			ClientCAs:    opts.CaPool,
			ClientAuth:   tls.RequireAndVerifyClientCert, // Use tls.NoClientCert if no mTLS needed
			MinVersion:   tls.VersionTLS12,
		}
		tlsConfig.BuildNameToCertificate()
		srv = &http.Server{
			Addr:      opts.Addr,
			TLSConfig: tlsConfig,
			Handler:   router,
		}

	} else {
		srv = &http.Server{
			Addr:    opts.Addr,
			Handler: h2c.NewHandler(router, &http2.Server{}),
		}

	}

	s := &Server{
		router: router,
		Srv:    srv,
	}

	s.AddRoutes("", opts.Routes)
	return s
}

// AddRoutes mounts routes on the server. It must not be called once the
// server is serving: gin's router is not safe to modify while it handles
// requests.
func (s *Server) AddRoutes(group string, routes []Route) {
	var router gin.IRouter = s.router
	if len(group) > 0 {
		router = router.Group(group)
	}
	for _, r := range routes {
		switch r.Method {
		case http.MethodGet:
			router.GET(r.Pattern, r.Handler)
		case http.MethodPost:
			router.POST(r.Pattern, r.Handler)
		case http.MethodPut:
			router.PUT(r.Pattern, r.Handler)
		case http.MethodPatch:
			router.PATCH(r.Pattern, r.Handler)
		case http.MethodDelete:
			router.DELETE(r.Pattern, r.Handler)
		default:
			router.Any(r.Pattern, r.Handler)
		}
	}
}

// Listen binds the server's address without accepting on it. Binding is where
// the interesting failures are (the port is taken, the interface does not
// exist), so a caller that will serve later can still surface them at
// start-up. Connections arriving before Serve wait in the kernel's backlog.
// Calling it again on a bound server is a no-op.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrServerStopped
	}
	if s.ln != nil {
		return nil
	}
	addr := s.Srv.Addr
	if addr == "" {
		if s.Srv.TLSConfig != nil {
			addr = ":https"
		} else {
			addr = ":http"
		}
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ln = ln
	return nil
}

// Serve begins accepting on the listener bound by Listen, in the background.
// It returns once accepting has started; mount every route before calling it.
func (s *Server) Serve() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.stopped:
		return ErrServerStopped
	case s.ln == nil:
		return ErrNotListening
	case s.serving:
		return ErrAlreadyServing
	}
	s.serving = true

	ln := s.ln
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// the only error left after a successful bind is the one Stop causes
		if s.Srv.TLSConfig != nil {
			//certificates come from TLSConfig, so no files are named
			s.Srv.ServeTLS(ln, "", "")
		} else {
			s.Srv.Serve(ln)
		}
	}()
	return nil
}

// Start binds and serves in one step.
func (s *Server) Start() error {
	if err := s.Listen(); err != nil {
		return err
	}
	return s.Serve()
}

// Addr is the bound address, or nil before Listen. It differs from Srv.Addr
// when that names port 0.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// Stop closes the server and its listener, including one that was bound and
// never served, which http.Server.Close would leave open because it only
// tracks listeners handed to Serve. A stopped server cannot be restarted.
func (s *Server) Stop() {
	s.mu.Lock()
	s.stopped = true
	if s.ln != nil && !s.serving {
		s.ln.Close()
	}
	s.mu.Unlock()

	s.Srv.Close()
	s.wg.Wait()
}
