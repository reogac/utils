package oam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v3"
)

// Requests at once each run on their own flag values, and --help is answered
// beside them. Every request used to parse into the one set of flag objects a
// context's table holds, and urfave/cli writes package-level state on every
// run, so concurrent requests raced - which -race reports - and one could run
// on another's values.
func TestConcurrentCommandsKeepTheirOwnFlagValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cmds := map[string]cli.Command{
		"echo": {
			Name:  "echo",
			Flags: []cli.Flag{&cli.IntFlag{Name: "id"}, &cli.IntFlag{Name: "n"}},
			Action: func(_ context.Context, cmd *cli.Command) error {
				fmt.Fprintf(cmd.Writer, "%d=%d", cmd.Int("id"), cmd.Int("n"))
				return nil
			},
		},
	}
	ctx := NewHandlerContext("root", nil, cmds, nil)
	engine := gin.New()
	for _, r := range NewOamHandler("test", "root", func(string) *HandlerContext { return ctx }).Routes() {
		engine.Handle(r.Method, "/"+r.Pattern, r.Handler)
	}
	run := func(args ...string) CmdResponse {
		body, _ := json.Marshal(CmdRequest{ContextId: "root", Name: "echo", Args: args})
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/"+OAM_CMD, bytes.NewReader(body)))
		var rsp CmdResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &rsp); err != nil {
			t.Errorf("response %q: %v", rec.Body.String(), err)
		}
		return rsp
	}

	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if got, want := run("--id", fmt.Sprint(i), "--n", fmt.Sprint(i*10)).Message, fmt.Sprintf("%d=%d", i, i*10); got != want {
				t.Errorf("request %d answered %q, want %q", i, got, want)
			}
		}()
		go func() {
			defer wg.Done()
			if rsp := run("--help"); !bytes.Contains([]byte(rsp.Message), []byte("--id")) {
				t.Errorf("help answered %+v", rsp)
			}
		}()
	}
	wg.Wait()
}
