package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/abiosoft/ishell"
	"github.com/reogac/utils/oam"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	shell     *ishell.Shell
	context   *Context
	serverUrl string
}

// NewClient creates and initializes a new CLI client
func NewClient() *Client {
	client := &Client{
		shell: ishell.New(),
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

// connect to remote OAM server
func (c *Client) connect(url string) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}
	req := &oam.ConnectionRequest{
		Nonce: 100,
	}
	reqBytes, _ := json.Marshal(req)

	httpRsp, err := http.Post(url+fmt.Sprintf("/%s/%s", oam.OAM_ROOT, oam.OAM_CONN), "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		c.shell.Printf("Failed to connect to server: %v\n", err)
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
	c.serverUrl = url
}

func (c *Client) requestCmd(cmd string, ctx *ishell.Context) {
	req := oam.CmdRequest{
		ContextId: c.context.id,
		Name:      cmd,
		Args:      ctx.Args,
	}
	reqBytes, _ := json.Marshal(req)

	httpRsp, err := http.Post(fmt.Sprintf("%s/%s/%s", c.serverUrl, oam.OAM_ROOT, oam.OAM_CMD), "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		ctx.Printf("Fail to send command: %+v\n", err)
		return
	}
	defer httpRsp.Body.Close()

	rspBytes, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		ctx.Printf("Failed to decode response: %+v\n", err)
		return
	}

	var rsp oam.CmdResponse
	if err := json.Unmarshal(rspBytes, &rsp); err != nil {
		ctx.Printf("Failed to parse response: %+v\nresponse body: %s\n", err, string(rspBytes))
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
