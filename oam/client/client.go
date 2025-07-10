package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/abiosoft/ishell"
	"github.com/reogac/utils/oam"
	"net/http"
	"strings"
)

type Client struct {
	shell   *ishell.Shell
	context *Context
	server  *ServerInfo
	cert    *tls.Certificate
	caPool  *x509.CertPool
}

// NewClient creates and initializes a new CLI client
func NewClient(cert *tls.Certificate, caPool *x509.CertPool) *Client {
	client := &Client{
		shell:  ishell.New(),
		cert:   cert,
		caPool: caPool,
	}
	return client
}

// Run starts the interactive shell
func (c *Client) Run() {
	c.shell.Println("Interactive CLI Client")
	c.goContext(oam.ServerContext{
		Id:     "root",
		Prompt: ">>>",
	})
	c.shell.Run()
}

func (c *Client) sendHttpRequest(req *http.Request) (*http.Response, error) {
	for k, v := range c.server.headers {
		req.Header.Add(k, v)
	}
	return c.server.cli.SendRequest(req)
}

// connect to remote OAM server
func (c *Client) connect(srv *ServerInfo) {
	c.server = srv
	req := &oam.ConnectionRequest{
		Nonce: 100,
	}
	reqBytes, _ := json.Marshal(req)

	var httpRsp *http.Response
	var httpReq *http.Request
	var err error

	if httpReq, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", srv.url, oam.OAM_CONN), bytes.NewBuffer(reqBytes)); err != nil {
		c.shell.Printf("Failed to create a http request: %+v\n", err)
		return
	}

	httpRsp, err = c.sendHttpRequest(httpReq)

	if err != nil {
		c.shell.Printf("Failed to connect to server: %+v\n", err)
		return
	}

	defer httpRsp.Body.Close()
	var rsp oam.ConnectionResponse
	if err := json.NewDecoder(httpRsp.Body).Decode(&rsp); err != nil {
		c.shell.Printf("Fail to decode response: %+v\n", err)
		return
	}
	if len(rsp.Error) > 0 {
		c.shell.Printf("Server return error: %s\n", rsp.Error)
		return
	}

	c.shell.Printf("Server %s replied: %s\n", rsp.ServerName, rsp.Message)
	c.goContext(rsp.Context)
}

func (c *Client) requestCmd(cmd string, ctx *ishell.Context) {
	req := oam.CmdRequest{
		ContextId: c.context.id,
		Name:      cmd,
		Args:      ctx.Args,
	}
	reqBytes, _ := json.Marshal(req)
	var httpReq *http.Request
	var httpRsp *http.Response
	var err error
	if httpReq, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", c.server.url, oam.OAM_CMD), bytes.NewBuffer(reqBytes)); err != nil {

		ctx.Printf("Fail to build an http request: %+v\n", err)
		return
	}
	if httpRsp, err = c.sendHttpRequest(httpReq); err != nil {
		ctx.Printf("Fail to send command: %+v\n", err)
		return
	}
	defer httpRsp.Body.Close()

	var rsp oam.CmdResponse

	if err := json.NewDecoder(httpRsp.Body).Decode(&rsp); err != nil {
		ctx.Printf("Failed to decode response: %+v\n", err)
		return
	}

	if rsp.Error != "" {
		ctx.Printf("Server error: %s\n", rsp.Error)
		return
	}

	if rsp.Context != nil {
		//move to new context
		c.goContext(*rsp.Context)
	} else {
		//print the server response
		ctx.Println(rsp.Message)
	}
}

// generateLongHelp creates detailed help for a command
func writeDetailHelp(cmd oam.CommandInfo) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(cmd.Name)

	if len(cmd.ArgsUsage) > 0 {
		sb.WriteString(" ")
		sb.WriteString(cmd.ArgsUsage)
	} else {
		sb.WriteString(" [command [command options]]")
	}
	sb.WriteString("\n")

	if len(cmd.Flags) > 0 {
		for _, flag := range cmd.Flags {
			sb.WriteString("   --")
			sb.WriteString(flag.Name)
			sb.WriteString(":  ")
			sb.WriteString(flag.Usage)
			if flag.DefaultText != "" {
				sb.WriteString(" (default: ")
				sb.WriteString(flag.DefaultText)
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
