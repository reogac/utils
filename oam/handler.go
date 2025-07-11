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
	NextContext() *HandlerContext
}

type HandlerContext struct {
	id      string //context identity
	cmds    map[string]cli.Command
	handler any
	ext     *HandlerContext
}

func NewHandlerContext(id string, h any, cmds map[string]cli.Command, extention *HandlerContext) *HandlerContext {
	return &HandlerContext{
		id:      id,
		cmds:    cmds,
		handler: h,
		ext:     extention,
	}
}

func (h *HandlerContext) prompt() string {
	return h.id + "> "
}
func (h *HandlerContext) buildServerContext() *ServerContext {
	return &ServerContext{
		Id:       h.id,
		Prompt:   h.prompt(),
		Commands: h.buildCommands(),
	}
}

func (h *HandlerContext) buildCommands() (infos []CommandInfo) {
	cmds := buildCommands(h.cmds)
	if h.ext != nil {
		cmds = append(cmds, h.ext.buildCommands()...)
	}
	return cmds
}

func (h *HandlerContext) handle(req *CmdRequest, c *gin.Context) {
	rsp := new(CmdResponse)
	if cmd, ok := h.cmds[req.Name]; !ok { //command not found, move to extention
		if h.ext != nil {
			h.ext.handle(req, c)
		} else { //or error
			rsp.Error = fmt.Sprintf("Unkown command: %s", req.Name)
			c.JSON(http.StatusInternalServerError, rsp)
		}
	} else {
		// attach handler
		ctx := context.WithValue(context.Background(), "handler", h.handler)

		//set writer to catch help/usage/error output
		var w bytes.Buffer
		cmd.Writer = &w
		cmd.ErrWriter = &w

		args := append([]string{req.Name}, req.Args...)
		// execute command
		if err := cmd.Run(ctx, args); err == nil {
			if buf := w.Bytes(); len(buf) > 0 {
				rsp.Message = string(buf)
			}
			//if the context handler set the next context, write it to the
			//response message
			if inf, ok := h.handler.(HasNextContext); ok {
				if nextCtx := inf.NextContext(); nextCtx != nil {
					//set the next context
					rsp.Context = nextCtx.buildServerContext()
				}
			}
			c.JSON(http.StatusOK, rsp)
		} else {
			rsp.Error = err.Error()
			c.JSON(http.StatusInternalServerError, rsp)
		}
	}

}

type OamHandler struct {
	name      string
	ctxGetter func(string) *HandlerContext
	rootId    string
}

func NewOamHandler(name string, rootId string, getter func(string) *HandlerContext) *OamHandler {
	return &OamHandler{
		name:      name,
		rootId:    rootId,
		ctxGetter: getter,
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
					"service": h.name,
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
	root := h.ctxGetter(h.rootId)
	if root == nil {
		c.JSON(http.StatusBadRequest, ConnectionResponse{
			Error: "Fail to get root context: " + h.rootId,
		})
		return
	}
	srvCtx := root.buildServerContext()
	// Prepare response
	c.JSON(http.StatusOK, ConnectionResponse{
		Nonce:      req.Nonce,
		ServerName: h.name,
		Message:    fmt.Sprintf("%s server connected, type help to see commands", h.name),
		Context:    *srvCtx,
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
	hCtx := h.ctxGetter(req.ContextId)
	if hCtx == nil {
		c.JSON(http.StatusInternalServerError, CmdResponse{
			Error: fmt.Sprintf("Unknown context %s", req.ContextId),
		})
	} else {
		//3. handle command
		hCtx.handle(&req, c)
	}
}
