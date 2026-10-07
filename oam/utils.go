package oam

import (
	"github.com/urfave/cli/v3"
	"strings"
)

func buildCommands(commands map[string]cli.Command) (infos []CommandInfo) {
	// convert cli command to CommandInfo
	for _, cmd := range commands {
		info := CommandInfo{
			Name:        cmd.Name,
			Usage:       cmd.Usage,
			Description: cmd.Description,
			ArgsUsage:   cmd.ArgsUsage,
		}

		for _, flag := range cmd.Flags {
			info.Flags = append(info.Flags, flagInfo(flag))
		}
		//TODO: add subcommands

		infos = append(infos, info)
	}

	return
}

// flagInfo describes a flag of any type through the interfaces urfave/cli's
// own help reads, so a duration or slice flag is described like a string one.
// A flag's required mark is advertised, not enforced here: cli decides that
// with IsSet, which stays true on a reused flag after its first request, so a
// server checks a required flag's value itself.
func flagInfo(flag cli.Flag) FlagInfo {
	info := FlagInfo{Name: strings.Join(flag.Names(), ", ")}
	if f, ok := flag.(cli.DocGenerationFlag); ok {
		info.Usage = f.GetUsage()
		if f.TakesValue() {
			info.DefaultText = f.GetDefaultText()
		}
	}
	if f, ok := flag.(cli.RequiredFlag); ok {
		info.Required = f.IsRequired()
	}
	return info
}
