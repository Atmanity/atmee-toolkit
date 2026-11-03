package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

const (
	minSpeakAfterS = 0.1
	maxSpeakAfterS = 10
)

func (a *app) conductorCmd() *cobra.Command {
	c := group("conductor", "v2: give an avatar's conversations a fixed agenda, a graph of steps run by Causeur",
		a.conductorSetCmd(), a.conductorGetCmd(), a.conductorDeleteCmd())
	c.Long = "A conductor is a fixed agenda for a v2 avatar's conversations: a graph of steps, each with the " +
		"avatar's instructions while the conversation is in it, and edges that say when it moves on. Causeur runs it. " +
		"It applies from the next session; nothing is rebuilt. v1 avatars have none. See docs/handbook/workflows.md."
	return c
}

func (a *app) conductorSetCmd() *cobra.Command {
	var graphPath string
	var speakAfter float64
	var jsonOut bool
	c := &cobra.Command{
		Use:   "set AVATAR_ID",
		Short: "Set (replace) a v2 avatar's conductor from a JSON file.",
		Long: "Set a v2 avatar's conductor, replacing any earlier one. --graph FILE holds either the graph itself " +
			"({\"start\": ..., \"nodes\": [...]}) or a whole request body ({\"graph\": {...}, \"speakAfterS\": 1.5}); " +
			"a top-level \"graph\" key marks the second. --speak-after replaces the file's speakAfterS. " +
			"`atmee avatars conductor get AVATAR_ID --out FILE` saves a file this command takes back.\n\n" +
			"The API checks the graph and names the field it refuses. It applies from the next session; " +
			"a running session keeps the conductor it started with.",
		Example: `  atmee avatars conductor set AVATAR_ID --graph agenda.json
  atmee avatars conductor set AVATAR_ID --graph agenda.json --speak-after 1.5
  cat agenda.json | atmee avatars conductor set AVATAR_ID --graph -`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if graphPath == "" {
				return usagef("--graph is required: a JSON file with the graph (- reads stdin)")
			}
			var speak *float64
			if cmd.Flags().Changed("speak-after") {
				// NaN fails both comparisons, so it is refused by name.
				if math.IsNaN(speakAfter) || speakAfter < minSpeakAfterS || speakAfter > maxSpeakAfterS {
					return usagef("--speak-after takes %g to %g seconds", minSpeakAfterS, float64(maxSpeakAfterS))
				}
				speak = &speakAfter
			}
			raw, err := a.readInput(graphPath)
			if err != nil {
				return err
			}
			body, err := api.ConductorBody(raw, speak)
			if err != nil {
				return usagef("--graph %s: %v", graphPath, err)
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			cd, err := cl.SetConductor(ctx(), args[0], body)
			if err != nil {
				return conductorError(err, args[0], graphPath, isBareGraph(raw))
			}
			if jsonOut {
				return a.printJSON(cd)
			}
			a.printf("Conductor set on %s.\n", args[0])
			a.printConductor(cd)
			a.printf("It applies from the next session.\n")
			return nil
		},
	}
	c.Flags().StringVar(&graphPath, "graph", "", "JSON file with the graph, or a {graph, speakAfterS} body (- reads stdin)")
	c.Flags().Float64Var(&speakAfter, "speak-after", 0, "Seconds before the avatar goes on along a speak edge, 0.1-10 (default: Causeur's, 0.5)")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) conductorGetCmd() *cobra.Command {
	var jsonOut bool
	var outPath string
	c := &cobra.Command{
		Use:   "get AVATAR_ID",
		Short: "Show a v2 avatar's conductor: its steps, edges and start.",
		Long: "Show a v2 avatar's conductor. --out FILE saves it as a request body ({\"graph\", \"speakAfterS\"}) " +
			"that `atmee avatars conductor set --graph FILE` takes back; --json prints the API's JSON.",
		Example: `  atmee avatars conductor get AVATAR_ID
  atmee avatars conductor get AVATAR_ID --out agenda.json`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			cd, err := cl.GetConductor(ctx(), args[0])
			if err != nil {
				return conductorError(err, args[0], "", false)
			}
			if outPath != "" {
				if err := writeConductorFile(outPath, cd); err != nil {
					return err
				}
				a.note("Saved to %s\n", outPath)
			}
			if jsonOut {
				return a.printJSON(cd)
			}
			a.printConductor(cd)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	c.Flags().StringVar(&outPath, "out", "", "Save it to this file as a {graph, speakAfterS} body for `conductor set`")
	return c
}

func (a *app) conductorDeleteCmd() *cobra.Command {
	var confirm bool
	c := &cobra.Command{
		Use:   "delete AVATAR_ID",
		Short: "Remove a v2 avatar's conductor (asks first; --yes skips the question).",
		Long: "Remove a v2 avatar's conductor. The next session runs without one, also without a conductor the " +
			"avatar's own config carries. Removing a conductor that is not set is no error.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				if err := a.confirm(fmt.Sprintf("Remove the conductor of avatar %s? [y/N]", args[0])); err != nil {
					return err
				}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			if err := cl.DeleteConductor(ctx(), args[0]); err != nil {
				return conductorError(err, args[0], "", false)
			}
			a.printf("Conductor removed from %s; the next session runs without one.\n", args[0])
			return nil
		},
	}
	c.Flags().BoolVar(&confirm, "yes", false, "Remove without asking")
	c.Flags().BoolVar(&confirm, "confirm", false, "Same as --yes")
	_ = c.Flags().MarkHidden("confirm")
	return c
}

// readInput reads a file, or stdin for "-".
func (a *app) readInput(path string) ([]byte, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(a.in)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, usagef("could not read the input file: %v", err)
	}
	return raw, nil
}

// isBareGraph says whether a --graph file held the graph itself rather than
// a {graph, speakAfterS} body (see api.ConductorBody).
func isBareGraph(raw []byte) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return false
	}
	_, full := top["graph"]
	return !full
}

func writeConductorFile(path string, cd *api.Conductor) error {
	body := map[string]any{"graph": cd.Graph}
	if cd.SpeakAfterS != nil {
		body["speakAfterS"] = *cd.SpeakAfterS
	}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("could not write %s: %v", path, err)
	}
	return nil
}

// conductorError says what a refusal means for the conductor commands; other
// errors pass through. graphPath and bare are set for a set: a refused
// field's path is also given as it reads in the file.
func conductorError(err error, avatarID, graphPath string, bare bool) error {
	var e *api.Error
	if !errors.As(err, &e) {
		return err
	}
	switch e.Code {
	case "not_v2_avatar":
		return fmt.Errorf("%s is a v1 avatar: the conductor runs only on v2 avatars (%s)", avatarID, e.Code)
	case "conductor_not_set":
		return fmt.Errorf("no conductor is set on %s (set one with `atmee avatars conductor set %s --graph FILE`)", avatarID, avatarID)
	case "avatar_not_found":
		return fmt.Errorf("avatar %s not found (find it with `atmee avatars list`)", avatarID)
	case "invalid_conductor":
		field, msg, ok := strings.Cut(e.Message, ": ")
		if !ok || !strings.HasPrefix(field, "/") {
			return fmt.Errorf("the API refused the graph (%s): %s", e.Code, e.Message)
		}
		where := field
		if inFile := strings.TrimPrefix(field, "/graph"); bare && graphPath != "" && inFile != field {
			if inFile == "" {
				inFile = "the whole graph"
			}
			where += " (" + inFile + " in " + graphPath + ")"
		}
		return fmt.Errorf("the API refused the graph at %s: %s (%s)", where, msg, e.Code)
	}
	if e.Status == 422 {
		return fmt.Errorf("the API refused the request (422): %s", e.Message)
	}
	return err
}

// conductorView is the part of a graph the summary reads.
type conductorView struct {
	Start string `json:"start"`
	Nodes []struct {
		ID           string `json:"id"`
		Instructions string `json:"instructions"`
		Edges        []struct {
			To    string  `json:"to"`
			When  *string `json:"when"`
			Speak bool    `json:"speak"`
		} `json:"edges"`
	} `json:"nodes"`
}

func (a *app) printConductor(cd *api.Conductor) {
	var g conductorView
	_ = json.Unmarshal(cd.Graph, &g)
	speak := "Causeur's default"
	if cd.SpeakAfterS != nil {
		speak = strconv.FormatFloat(*cd.SpeakAfterS, 'f', -1, 64) + " s"
	}
	a.printf("%s, %s, start node %s, speak after %s\n", plural(cd.Nodes, "node"), plural(cd.Edges, "edge"), orDefault(g.Start, "-"), speak)
	for _, n := range g.Nodes {
		var to []string
		for _, e := range n.Edges {
			t := e.To
			if e.When != nil && strings.TrimSpace(*e.When) != "" {
				t += " (when)"
			}
			if e.Speak {
				t += " (speak)"
			}
			to = append(to, t)
		}
		next := "end"
		if len(to) > 0 {
			next = "-> " + strings.Join(to, ", ")
		}
		a.printf("  %s %s\n", n.ID, next)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
