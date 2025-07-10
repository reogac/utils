package oam

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/reogac/utils/httpw"
	"github.com/sirupsen/logrus"
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

//inteface for any context handler (AmfHandler/UeHandler)
type ContextHandler interface {
	HandleCmd(string, []string) (CmdResponse, error)
}

type OamApiBackend interface {
	GetName() string
	GetContext() ServerContext
	GetHandler(string) ContextHandler //get handler for given context id
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
		Context:    h.backend.GetContext(),
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
	ctxHandler := h.backend.GetHandler(req.ContextId)
	if ctxHandler == nil {
		c.JSON(http.StatusInternalServerError, CmdResponse{
			Error: fmt.Sprintf("Unknown context %s", req.ContextId),
		})

		return
	}
	// 3. Execute the command with handler
	rsp, err := ctxHandler.HandleCmd(req.Name, req.Args)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CmdResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, rsp)

}
