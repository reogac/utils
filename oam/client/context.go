package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/abiosoft/ishell"
	"github.com/reogac/utils/oam"
	"github.com/urfave/cli/v3"
	"os"
)

type Context struct {
	id       string //context id
	prompt   string
	parent   *Context //parent context
	commands []ishell.Cmd
}

func (c *Client) displayContextHelp(ctx *ishell.Context) {
	for _, cmd := range c.context.commands {
		ctx.Printf(" - %s: %s\n", cmd.Name, cmd.Help)
	}
}

func (c *Client) goBack(ctx *ishell.Context) {
	if c.context.parent != nil {
		// move back to parent context
		c.setContext(c.context.parent)
	} else { //exit
		ctx.Println("Goodbye!")
		os.Exit(0)
	}
}
func (c *Client) setContext(ctx *Context) {
	// delete current commands
	if c.context != nil {
		for _, cmd := range c.context.commands {
			//c.shell.Println("Delete cmd " + cmd.Name)
			c.shell.DeleteCmd(cmd.Name)
		}
	}
	c.shell.SetPrompt(ctx.prompt + " ")
	// add commands; display help
	for _, cmd := range ctx.commands {
		//c.shell.Println("Add cmd " + cmd.Name)
		c.shell.AddCmd(&cmd)
		c.shell.Printf(" - %s: %s\n", cmd.Name, cmd.Help)
	}
	// and set context
	c.context = ctx
}

func (c *Client) goContext(ctxInfo oam.ServerContext) {
	if len(ctxInfo.Id) == 0 {
		c.shell.Println("Empty context")
		return
	}
	//set prompt
	prompt := ctxInfo.Prompt
	if len(prompt) == 0 {
		prompt = ">>>"
	}

	var newCtx *Context
	commands := []ishell.Cmd{
		ishell.Cmd{
			Name: "help",
			Help: "display Help",
			Func: func(ctx *ishell.Context) {
				c.displayContextHelp(ctx)
			},
		},
		ishell.Cmd{
			Name: "clear",
			Help: "clear the screen",
			Func: func(ctx *ishell.Context) {
				ctx.ClearScreen()
			},
		},
	}

	//create new context then set it at current
	if ctxInfo.Id == "root" { //root context
		commands = append(commands, ishell.Cmd{
			Name: "exit",
			Help: "exit the program",
			Func: func(ctx *ishell.Context) {
				c.goBack(ctx)
			},
		})
		commands = append(commands, ishell.Cmd{
			Name: "connect",
			Help: "Connect to a remote server",
			Func: func(ctx *ishell.Context) {
				var w bytes.Buffer
				var connectCmd *cli.Command = &cli.Command{
					Name:        "connect",
					Usage:       "Connect to an OAM server",
					Description: "Connect to an OAM server",
					Arguments: []cli.Argument{
						&cli.StringArg{
							Name: "url",
						},
					},
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "headers",
							Usage: "List of key:value headers to send to server",
						},
						&cli.StringFlag{
							Name:  "certName",
							Usage: "Subject name on the server certifice for TLS verification",
						},
					},
					Action: func(ctx context.Context, cmd *cli.Command) error {
						c := ctx.Value("client").(*Client)
						url := cmd.StringArg("url")
						if len(url) == 0 {
							return fmt.Errorf("server url is missing")
						}
						c.connect(url, cmd.String("headers"), cmd.String("certName"))
						return nil
					},
					Writer:    &w,
					ErrWriter: &w,
				}

				args := append([]string{"connect"}, ctx.Args...)
				cmdCtx := context.WithValue(context.Background(), "client", c)

				if err := connectCmd.Run(cmdCtx, args); err == nil {
					if buf := w.Bytes(); len(buf) > 0 {
						ctx.Printf("%s\n", string(buf))
					}
				} else {
					ctx.Printf("Fail to connect: %+v\n", err)
				}
			},
		})
		newCtx = &Context{
			id: ctxInfo.Id,
		}

	} else {

		commands = append(commands, ishell.Cmd{
			Name: "exit",
			Help: "exit current context ",
			Func: func(ctx *ishell.Context) {
				c.goBack(ctx)
			},
		})
		for _, cmd := range ctxInfo.Commands {
			commands = append(commands, ishell.Cmd{
				Name:     cmd.Name,
				Help:     cmd.Usage,
				LongHelp: writeDetailHelp(cmd),
				Func: func(ctx *ishell.Context) {
					c.requestCmd(cmd.Name, ctx)
				},
			})
		}
		newCtx = &Context{
			id:     ctxInfo.Id,
			parent: c.context,
		}

	}
	newCtx.prompt = prompt
	newCtx.commands = commands
	c.setContext(newCtx)
}
