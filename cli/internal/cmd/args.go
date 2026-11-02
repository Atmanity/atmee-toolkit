package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// argHints say where each positional argument comes from, so a missing one
// is an error that tells you what to run instead of "accepts 1 arg(s)".
var argHints = map[string]string{
	"AVATAR_ID":    "find it with `atmee avatars list`",
	"CANDIDATE_ID": "find it with `atmee avatars candidates list AVATAR_ID`",
	"SESSION_ID":   "the sessionId printed when the session started",
	"PATH":         "a local file",
	"MICROCREDITS": "an amount; 100 microcredits = 1 credit",
	"KEY_ID":       "find it with `atmee keys list`",
	"VERSION_ID":   "find it with `atmee avatars candidates list AVATAR_ID --all`",
	"MESSAGE":      "what to say, or - to read it from stdin",
}

// argNames are the positional arguments a command's Use line names, in order
// (optional ones in brackets included).
func argNames(c *cobra.Command) []string {
	fields := strings.Fields(c.Use)
	if len(fields) == 0 {
		return nil
	}
	return fields[1:]
}

func argProblem(c *cobra.Command, got, want int) error {
	names := argNames(c)
	if got < want && got < len(names) {
		missing := strings.Trim(names[got], "[].")
		msg := "missing " + missing
		if hint, ok := argHints[missing]; ok {
			msg += " (" + hint + ")"
		}
		return usagef("%s\nUsage: %s", msg, c.UseLine())
	}
	return usagef("too many arguments\nUsage: %s", c.UseLine())
}

// exactArgs is cobra.ExactArgs with an error that names the argument.
func exactArgs(n int) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) != n {
			return argProblem(c, len(args), n)
		}
		return nil
	}
}

// minArgs is cobra.MinimumNArgs with an error that names the argument.
func minArgs(n int) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) < n {
			return argProblem(c, len(args), n)
		}
		return nil
	}
}
