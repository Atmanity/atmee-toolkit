package cmd

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Atmanity/toolkit/cli/internal/api"
	"github.com/Atmanity/toolkit/cli/internal/creds"
)

// Variant is one build of this CLI: what it is called, the API it uses by
// default and where its login lives, so that two builds can share a machine.
// Each has its own environment variables and reads no other build's, so a
// shell set up for one can't send its server or key to the other.
type Variant struct {
	Name       string // the command's name, in help, usage and hints
	BaseURL    string
	BaseURLEnv string // overrides BaseURL
	HomeFolder string // under the home directory
	HomeEnv    string // overrides HomeFolder
	APIKeyEnv  string
	// Extra are commands only this build has, added beside the shared ones.
	Extra []CommandFunc
}

// CommandFunc builds one command of a build.
type CommandFunc func(*app) *cobra.Command

var defaultVariant = Variant{
	Name:       "atmee",
	BaseURL:    api.DefaultBaseURL,
	BaseURLEnv: "ATMEE_BASE_URL",
	HomeFolder: ".atmee",
	HomeEnv:    creds.DefaultHomeEnv,
	APIKeyEnv:  creds.DefaultKeyEnv,
}

// renamer returns what turns the commands' text, written for atmee, into the
// variant's: its command lines and its environment variables. ATMEE_API_KEY=...
// stays: that is the line `keys create --out` writes for an app to read.
func (v Variant) renamer(root *cobra.Command) func(string) string {
	if v.Name == defaultVariant.Name {
		return func(s string) string { return s }
	}
	var subs []string
	for _, c := range root.Commands() {
		subs = append(subs, regexp.QuoteMeta(c.Name()))
	}
	line := regexp.MustCompile(`\batmee (` + strings.Join(subs, "|") + `)\b`)
	d := defaultVariant
	names := strings.NewReplacer(
		"`"+d.Name+"`", "`"+v.Name+"`",
		d.APIKeyEnv+"=", d.APIKeyEnv+"=",
		d.APIKeyEnv, v.APIKeyEnv,
		d.BaseURLEnv, v.BaseURLEnv,
		d.HomeEnv, v.HomeEnv,
	)
	return func(s string) string { return names.Replace(line.ReplaceAllString(s, v.Name+" ${1}")) }
}

// renameTree renames the help text of a command and everything under it.
func renameTree(c *cobra.Command, rename func(string) string) {
	c.Short, c.Long, c.Example = rename(c.Short), rename(c.Long), rename(c.Example)
	c.Flags().VisitAll(func(f *pflag.Flag) { f.Usage = rename(f.Usage) })
	for _, sub := range c.Commands() {
		renameTree(sub, rename)
	}
}
