package oam

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/reogac/utils/httpw"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"net/http"
)

var log *logrus.Entry

func init() {
	log = logrus.WithFields(logrus.Fields{"mod": "oam"})
}

const (
	OAM_STATUS string = "status"
	OAM_CONN   string = "connect"
	OAM_CMD    string = "cmd"
)

type HasNextContext interface {
	NextContext() *ServerContext
}

type OamApiBackend interface {
	GetName() string
	RootContext() ServerContext
	GetContextHandler(string) (any, map[string]cli.Command)
}

type OamHandler struct {
	backend OamApiBackend
}

func NewOamHandler(backend OamApiBackend) *OamHandler {
	return &OamHandler{
		backend: backend,
	}
}

func (h *OamHandler) Routes() []httpw.Route {
	return []httpw.Route{
		httpw.Route{
			Method:  http.MethodGet,
			Pattern: OAM_STATUS,
			Handler: func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "NF's OAM API ready",
					"service": h.backend.GetName(),
				})
			},
		},
		httpw.Route{
			Method:  http.MethodPost,
			Pattern: OAM_CONN,
			Handler: h.handleConn,
		},
		httpw.Route{
			Method:  http.MethodPost,
			Pattern: OAM_CMD,
			Handler: h.handleCmd,
		},
	}
}

// handle connection
func (h *OamHandler) handleConn(c *gin.Context) {
	log.Infof("Receive OAM client connection request")
	var req ConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ConnectionResponse{
			Error: "Invalid request format: " + err.Error(),
		})
		return
	}

	// Prepare response
	c.JSON(http.StatusOK, ConnectionResponse{
		Nonce:      req.Nonce,
		ServerName: h.backend.GetName(),
		Message:    fmt.Sprintf("%s server connected, type help to see commands", h.backend.GetName()),
		Context:    h.backend.RootContext(),
	})

}

// handle connection
func (h *OamHandler) handleCmd(c *gin.Context) {
	//1. decode request
	var req CmdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CmdResponse{
			Error: "Invalid request format: " + err.Error(),
		})
		return
	}
	//2. get context handler
	ctxHandler, cmds := h.backend.GetContextHandler(req.ContextId)
	if ctxHandler == nil {
		c.JSON(http.StatusInternalServerError, CmdResponse{
			Error: fmt.Sprintf("Unknown context %s", req.ContextId),
		})
		return
	}
	rsp := new(CmdResponse)
	// 3. Execute the command with handler
	if cmd, ok := cmds[req.Name]; !ok {
		rsp.Error = fmt.Sprintf("Unkown command: %s", req.Name)
		c.JSON(http.StatusInternalServerError, rsp)
	} else {
		// attach handler
		ctx := context.WithValue(context.Background(), "handler", ctxHandler)

		//set writer to catch help/usage/error output
		var w bytes.Buffer
		cmd.Writer = &w
		cmd.ErrWriter = &w

		args := append([]string{req.Name}, req.Args...)
		if err := cmd.Run(ctx, args); err == nil {
			if buf := w.Bytes(); len(buf) > 0 {
				rsp.Message = string(buf)
			}
			//if the context handler set the next context, write it to the
			//response message
			if inf, ok := ctxHandler.(HasNextContext); ok {
				//set the next context
				rsp.Context = inf.NextContext()
			}
			c.JSON(http.StatusOK, rsp)
		} else {
			rsp.Error = err.Error()
			c.JSON(http.StatusInternalServerError, rsp)
		}
	}
}
