package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

// App tools on `avatars session chat`: the tool list and the instructions
// come from files.

type appToolFlags struct {
	toolsFile, instructionsFile string
}

func (t *appToolFlags) register(c *cobra.Command) {
	f := c.Flags()
	f.StringVar(&t.toolsFile, "tools", "", "A JSON file with the tools the avatar may call (an array of function tools)")
	f.StringVar(&t.instructionsFile, "tool-instructions", "", "A text file (- for stdin): one text for all the tools, saying how and when the avatar uses them")
}

// apply reads the files into the session start and refuses the obvious
// mistakes before anything is sent; the API checks the tools' rules.
func (t *appToolFlags) apply(in *api.EmbedSessionInput, stdin io.Reader) error {
	if t.toolsFile != "" {
		raw, err := os.ReadFile(t.toolsFile)
		if err != nil {
			return usagef("--tools: %v", err)
		}
		tools, err := api.ParseTools(raw)
		if err != nil {
			return usagef("--tools %s: %v", t.toolsFile, err)
		}
		in.Tools = tools
	}
	if t.instructionsFile != "" {
		if t.toolsFile == "" {
			return usagef("--tool-instructions needs --tools (it says how the avatar uses those tools)")
		}
		var raw []byte
		var err error
		if t.instructionsFile == "-" {
			raw, err = io.ReadAll(stdin)
		} else {
			raw, err = os.ReadFile(t.instructionsFile)
		}
		if err != nil {
			return usagef("--tool-instructions: %v", err)
		}
		s := strings.TrimSpace(string(raw))
		if s == "" {
			return usagef("--tool-instructions %s is empty", t.instructionsFile)
		}
		in.Instructions = &s
	}
	return nil
}
