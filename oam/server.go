package oam

import (
	"github.com/reogac/utils/httpw"
)

type OamServer interface {
	Stop()
}

func StartOamServer(addr string, backend OamApiBackend) (srv OamServer, err error) {
	// create remote http server
	httpSrv := httpw.NewServer(httpw.Options{
		Addr:   addr,
		Routes: NewOamHandler(backend).Routes(), //add routes for handling remote requests
	})

	//start the http server
	if err = httpSrv.Start(); err == nil {
		srv = httpSrv
	}
	return
}
