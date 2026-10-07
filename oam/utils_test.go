package oam

import (
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

// Every flag type is described, not only string, bool and integer ones: a
// duration or slice flag used to arrive with no usage, and a required flag as
// optional.
func TestEveryFlagTypeIsDescribed(t *testing.T) {
	infos := buildCommands(map[string]cli.Command{"wait-for": {
		Name: "wait-for",
		Flags: []cli.Flag{
			&cli.DurationFlag{Name: "wait", Usage: "how long", Value: 5 * time.Second},
			&cli.StringSliceFlag{Name: "ue", Usage: "which UEs"},
			&cli.StringFlag{Name: "gnb", Usage: "which gNB", Required: true},
			&cli.BoolFlag{Name: "all", Usage: "every one"},
		},
	}})
	want := map[string]FlagInfo{
		"wait": {Name: "wait", Usage: "how long", DefaultText: "5s"},
		"ue":   {Name: "ue", Usage: "which UEs"},
		"gnb":  {Name: "gnb", Usage: "which gNB", Required: true},
		"all":  {Name: "all", Usage: "every one"},
	}
	for _, f := range infos[0].Flags {
		if w := want[f.Name]; f != w {
			t.Errorf("flag %s described as %+v, want %+v", f.Name, f, w)
		}
	}
}
