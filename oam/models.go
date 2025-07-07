package oam

// CommandInfo - Define the structure command same as server
type CommandInfo struct {
	Name        string        `json:"name"`
	Usage       string        `json:"usage"`
	Description string        `json:"description"`
	ArgsUsage   string        `json:"argsUsage"`
	Flags       []FlagInfo    `json:"flags"`
	Subcommands []CommandInfo `json:"subcommands,omitempty"`
}

// FlagInfo - Define the structure of flag same as server
type FlagInfo struct {
	Name        string `json:"name"`
	Usage       string `json:"usage"`
	DefaultText string `json:"defaultText,omitempty"`
	Required    bool   `json:"required"`
}

// A context at server
type ServerContext struct {
	Prompt   string
	Id       string
	Commands []CommandInfo
}

// First request to server
type ConnectionRequest struct {
	Nonce int
}

type ConnectionResponse struct {
	Nonce      int
	Error      string
	ServerName string
	Message    string
	Context    ServerContext
}

type CmdRequest struct {
	ContextId string
	Name      string //command name
	Args      []string
}

type CmdResponse struct {
	Message string
	Error   string
	Context *ServerContext //for switching to a new context
}
