package httpw

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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
	Addr   string
	Cert   tls.Certificate
	CaPool *x509.CertPool
	Routes []Route
}

type Server struct {
	Srv    *http.Server
	router gin.IRouter
	wg     sync.WaitGroup
}

func NewServer(opts Options) *Server {

	router := gin.New()

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
	var tlsConfig *tls.Config
	if opts.CaPool != nil {
		// Configure TLS with optional client certificate validation
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{opts.Cert},
			ClientCAs:    opts.CaPool,
			ClientAuth:   tls.RequireAndVerifyClientCert, // Use tls.NoClientCert if no mTLS needed
			MinVersion:   tls.VersionTLS12,
		}
		tlsConfig.BuildNameToCertificate()
	}

	s := &Server{
		router: router,
		Srv: &http.Server{
			Addr:      opts.Addr,
			TLSConfig: tlsConfig,
			Handler:   router,
		},
	}

	s.AddRoutes("", opts.Routes)
	return s
}

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

func (s *Server) Start() error {
	errCh := make(chan error, 1)
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		if s.Srv.TLSConfig != nil {
			errCh <- s.Srv.ListenAndServeTLS("", "")
		} else {
			errCh <- s.Srv.ListenAndServe()
		}
	}()

	t := time.NewTimer(100 * time.Millisecond)
	select {
	case <-t.C:
	case err := <-errCh:
		return err
	}
	return nil
}

func (s *Server) Stop() {
	s.Srv.Close()
	s.wg.Wait()
}
