package oam

import (
	"github.com/reogac/utils/httpw"
)

type OamServer interface {
	Stop()
}

func StartOamServer(addr, name, rootId string, getter func(string) *HandlerContext) (srv OamServer, err error) {
	// create remote http server
	httpSrv := httpw.NewServer(httpw.Options{
		Addr:   addr,
		Routes: NewOamHandler(name, rootId, getter).Routes(), //add routes for handling remote requests
	})

	//start the http server
	if err = httpSrv.Start(); err == nil {
		srv = httpSrv
	}
	return
}
