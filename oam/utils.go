package oam

import (
	"fmt"
	"github.com/urfave/cli/v3"
	"strings"
)

func BuildCommands(commands map[string]cli.Command) (infos []CommandInfo) {
	// convert cli command to CommandInfo
	for _, cmd := range commands {
		info := CommandInfo{
			Name:        cmd.Name,
			Usage:       cmd.Usage,
			Description: cmd.Description,
			ArgsUsage:   cmd.ArgsUsage,
		}

		// Process flags
		for _, flag := range cmd.Flags {
			flagInfo := FlagInfo{
				Name:  strings.Join(flag.Names(), ", "),
				Usage: flagUsage(flag),
			}

			// Set default value text based on flag type
			switch f := flag.(type) {
			case *cli.StringFlag:
				flagInfo.DefaultText = f.Value
			case *cli.BoolFlag:
				if f.Value {
					flagInfo.DefaultText = "true"
				} else {
					flagInfo.DefaultText = "false"
				}
			case *cli.IntFlag:
				if f.Value != 0 {
					flagInfo.DefaultText = fmt.Sprintf("%d", f.Value)
				}
			}

			info.Flags = append(info.Flags, flagInfo)
		}

		infos = append(infos, info)
	}

	return
}

func flagUsage(flag cli.Flag) string {
	switch f := flag.(type) {
	case *cli.StringFlag:
		return f.Usage
	case *cli.BoolFlag:
		return f.Usage
	case *cli.IntFlag:
		return f.Usage
	default:
		return ""
	}
}
