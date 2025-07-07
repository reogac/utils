package oam

import (
	"github.com/reogac/utils/httpw"
)

type OamServer interface {
	Stop()
}

func StartOamServer(addr string, backend OamApiBackend) (srv OamServer, err error) {
	// create remote http server
	httpSrv := httpw.New(httpw.Options{
		Addr: addr,
	})

	//add routes for handling remote requests
	httpSrv.AddRoutes(OAM_ROOT, NewOamHandler(backend).Routes())
	//start the http server
	if err = httpSrv.Start(); err == nil {
		srv = httpSrv
	}
	return
}
