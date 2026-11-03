// Package cmd is the `atmee` command line.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/creds"
)

// Version is set at build time (-ldflags "-X .../cmd.Version=..."); without
// that, it is the module version `go install ...@vX.Y.Z` records.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}

// usageError exits 2 (bad input), like a flag error.
type usageError struct{ error }

func usagef(format string, a ...any) error { return usageError{fmt.Errorf(format, a...)} }

// app is what every command shares; tests build one with their own streams.
type app struct {
	out, err io.Writer
	in       io.Reader
	store    creds.Store
	variant  Variant
	// rename puts the variant's names into text meant for people.
	rename func(string) string
	// mockScenario is the hidden --mock-scenario of the avatar commands.
	mockScenario string
}

func (a *app) baseURL() string {
	if u := strings.TrimSpace(os.Getenv(a.variant.BaseURLEnv)); u != "" {
		return u
	}
	return a.variant.BaseURL
}

func (a *app) api() (*api.Client, error) {
	cred, err := a.credential()
	if err != nil {
		return nil, err
	}
	c, err := api.New(a.baseURL(), cred, 60*time.Second)
	if err != nil {
		return nil, err
	}
	c.MockScenario = a.mockScenario
	c.JoinTimeout = joinTimeout
	c.WarmingTimeout = joinTimeout
	c.OnWarming = func(wait time.Duration) {
		a.note("The avatar is warming up on a rendering node; retrying in %ds\n", int(wait.Seconds()))
	}
	return c, nil
}

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printf renames the format only: the values are the user's or the API's.
func (a *app) printf(format string, args ...any) { fmt.Fprintf(a.out, a.rename(format), args...) }

// note goes to stderr so --json output stays clean.
func (a *app) note(format string, args ...any) { fmt.Fprintf(a.err, a.rename(format), args...) }

// prompt reads one line; hidden input when stdin is a terminal.
// confirm asks a yes/no question; anything but y or yes is "aborted".
func (a *app) confirm(question string) error {
	ans, err := a.prompt(question, false)
	if err != nil {
		return err
	}
	if !strings.EqualFold(ans, "y") && !strings.EqualFold(ans, "yes") {
		return usagef("aborted")
	}
	return nil
}

func (a *app) prompt(label string, hidden bool) (string, error) {
	fmt.Fprint(a.err, label+": ")
	if f, ok := a.in.(*os.File); ok && hidden && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.err)
		return strings.TrimSpace(string(b)), err
	}
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func ctx() context.Context { return context.Background() }

// pollInterval is how often waits poll; tests shorten it.
var pollInterval = 3 * time.Second

// joinTimeout bounds an avatar-only session start's join wait (per attempt)
// and, separately, its warm-up retries; tests shorten it.
var joinTimeout = api.JoinWaitTimeout

// NewRoot builds the command tree.
func NewRoot(out, errOut io.Writer, in io.Reader) *cobra.Command {
	return newApp(defaultVariant, out, errOut, in).root()
}

func newApp(v Variant, out, errOut io.Writer, in io.Reader) *app {
	return &app{
		out: out, err: errOut, in: in, variant: v,
		store:  creds.In(v.HomeFolder, v.HomeEnv, v.APIKeyEnv),
		rename: func(s string) string { return s },
	}
}

// root builds the command tree, under the variant's names.
func (a *app) root() *cobra.Command {
	root := a.commands()
	a.rename = a.variant.renamer(root)
	renameTree(root, a.rename)
	return root
}

// commands builds the command tree, its text written for atmee.
func (a *app) commands() *cobra.Command {
	root := &cobra.Command{
		Use:   a.variant.Name,
		Short: "Atmee CLI — create and manage conversational avatars.",
		Long: "Atmee CLI — create conversational avatars and talk to them.\n\n" +
			"Every command calls the Atmee API at ATMEE_BASE_URL (default " + a.variant.BaseURL + "; " +
			"a local mock is http://localhost:8091). Sign in first with " +
			"`atmee login` (browser), or use an API key (`atmee login --api-key` or ATMEE_API_KEY).",
		Example: `  atmee login
  atmee balance
  atmee avatars create --name Ada --image ./portrait.jpg --voice ./voice.m4a --wait
  atmee avatars session chat AVATAR_ID --open`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(a.out)
	root.SetErr(a.err)
	root.SetIn(a.in)
	root.AddCommand(a.loginCmd(), a.whoamiCmd(), a.logoutCmd(), a.balanceCmd(), a.accountCmd(), a.redeemCmd(), a.keysCmd(), a.avatarsCmd(), a.speechCmd())
	root.AddCommand(a.earningsCmd(), a.generateCmd())
	for _, extra := range a.variant.Extra {
		root.AddCommand(extra(a))
	}
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute(args []string, out, errOut io.Writer, in io.Reader) int {
	return ExecuteAs(defaultVariant, args, out, errOut, in)
}

// ExecuteAs runs the CLI as a variant and returns the process exit code.
func ExecuteAs(v Variant, args []string, out, errOut io.Writer, in io.Reader) int {
	a := newApp(v, out, errOut, in)
	root := a.root()
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	// Errors arrive whole, and their hints are built with the ids in them.
	// An engine or plan refusal gets what to do about it here, wherever it
	// came from (a create, an update, a capture).
	msg := err.Error()
	if h := engineHint(cmd, err); h != "" {
		msg += "; " + h
	}
	fmt.Fprintf(errOut, "Error: %s\n", a.rename(msg))
	var u usageError
	if errors.As(err, &u) || strings.HasPrefix(err.Error(), "unknown ") || strings.Contains(err.Error(), "required flag") ||
		strings.Contains(err.Error(), "accepts ") || strings.Contains(err.Error(), "invalid argument") {
		return 2
	}
	return 1
}

// group is a command that only holds subcommands: an unknown one is a usage
// error (cobra would otherwise print help and exit 0).
func group(use, short string, subs ...*cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
	}
	c.AddCommand(subs...)
	return c
}
