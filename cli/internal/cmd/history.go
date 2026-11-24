package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// The account's generation history: list, count and delete what generate
// made. Reading charges nothing; a delete refunds nothing.

const historyNote = "Any member of the account sees every generation of the account; an API key scoped to an avatar only its avatar's. " +
	"A signed-in user, or an API key with generation:write, avatars:read or avatars:write."

// historyFlags are the filters list and summary share.
type historyFlags struct {
	madeBy, statuses []string
	since, query     string
}

func (h *historyFlags) add(c *cobra.Command) {
	f := c.Flags()
	f.StringSliceVar(&h.madeBy, "made-by", nil, "Only generations made by any of these: me, apiKeys, former, or a member's user id or an API key id (`atmee generate summary` lists them; comma-separated)")
	f.StringSliceVar(&h.statuses, "status", nil, "Only generations in any of these states: rendering, ready, expiringSoon (ending within 3 days), expired, deleted (comma-separated)")
	f.StringVar(&h.since, "since", "", "Only generations started since then: a date (2026-11-01), an RFC 3339 time or a span back from now (72h, 7d)")
	f.StringVarP(&h.query, "query", "q", "", "Only generations whose prompt, script or description contains this text, ignoring case")
}

// filter checks the shared filters before any request.
func (h *historyFlags) filter(now time.Time) (api.GenerationFilter, error) {
	f := api.GenerationFilter{MadeBy: h.madeBy, Statuses: h.statuses, Query: h.query}
	for _, s := range h.statuses {
		if !slices.Contains(api.GenerationStatuses, s) {
			return f, usagef("--status takes %s (%s)", strings.Join(api.GenerationStatuses, ", "), s)
		}
	}
	if utf8.RuneCountInString(h.query) > 200 {
		return f, usagef("--query takes at most 200 characters")
	}
	if h.since != "" {
		t, err := parseSince(h.since, now)
		if err != nil {
			return f, err
		}
		f.Since = &t
	}
	return f, nil
}

// parseSince reads a date, an RFC 3339 time, or a span back from now in Go's
// duration form or in whole days (7d).
func parseSince(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		if n, err := strconv.Atoi(days); err == nil && n >= 0 {
			return now.AddDate(0, 0, -n), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil && d >= 0 {
		return now.Add(-d), nil
	}
	return time.Time{}, usagef("--since takes a date (2026-11-01), an RFC 3339 time or a span back from now (72h, 7d); got %q", s)
}

func (a *app) generateListCmd() *cobra.Command {
	var h historyFlags
	var kind, cursor string
	var modes []string
	var limit int
	var all, jsonOut bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List the account's clips, portraits and voices, newest first.",
		Long: "List the account's generations, newest first: every clip, portrait and voice call that finished (charged), and every one " +
			"still running. A flagged, refused or failed call charged nothing and is not listed. Each shows who made it, what it was made " +
			"from, what it charged and its outputs, with a fresh download link for each ready one. A clip is kept 30 days, a portrait or " +
			"voice 7; an output used by an avatar reads used. --all fetches every page; --cursor continues from the next cursor a page " +
			"printed, with the same filters. " + historyNote,
		Example: `  atmee generate list
  atmee generate list --kind clip --status ready,expiringSoon
  atmee generate list --made-by me --since 7d -q beach
  atmee generate list --all --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := h.filter(time.Now())
			if err != nil {
				return err
			}
			if kind != "" && !slices.Contains(api.GenerationKinds, kind) {
				return usagef("--kind takes clip, portrait or voice (%s)", kind)
			}
			for _, m := range modes {
				if !slices.Contains(api.GenerationModes, m) {
					return usagef("--mode takes picture, avatar or unknown (%s)", m)
				}
			}
			if len(modes) > 0 && kind != "" && kind != "clip" {
				return usagef("--mode is for clips: leave out --kind or pass --kind clip")
			}
			if limit < 0 || limit > 50 {
				return usagef("--limit takes 1 to 50")
			}
			if all && cursor != "" {
				return usagef("--all fetches every page from the first: leave out --cursor")
			}
			f.Kind, f.Modes = kind, modes
			cl, err := a.api()
			if err != nil {
				return err
			}
			page, err := cl.Generations(ctx(), f, limit, cursor)
			if err != nil {
				return err
			}
			items := page.Items
			for all && page.NextCursor != nil {
				if page, err = cl.Generations(ctx(), f, limit, *page.NextCursor); err != nil {
					return err
				}
				items = append(items, page.Items...)
			}
			if jsonOut {
				if all {
					return a.printJSON(api.GenerationPage{Items: items})
				}
				return a.printJSON(page)
			}
			if len(items) == 0 {
				a.printf("No generations.\n")
				return nil
			}
			for i, g := range items {
				if i > 0 {
					a.printf("\n")
				}
				a.printGeneration(g)
			}
			if !all && page.NextCursor != nil {
				a.printf("\nMore: atmee generate list --cursor %s (with the same filters)\n", *page.NextCursor)
			}
			return nil
		},
	}
	h.add(c)
	f := c.Flags()
	f.StringVar(&kind, "kind", "", "Only generations of this kind: clip, portrait or voice")
	f.StringSliceVar(&modes, "mode", nil, "Only clips made these ways: picture (from an image), avatar (a saved avatar speaking), unknown (made before modes were recorded); implies --kind clip")
	f.IntVar(&limit, "limit", 0, "Generations per page, 1 to 50 (the API's default is 24)")
	f.StringVar(&cursor, "cursor", "", "The next cursor a previous page printed")
	f.BoolVar(&all, "all", false, "Fetch every page")
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// printGeneration prints one generation: when, what and its state, who made
// it and what it charged, what it was made from, and its outputs.
func (a *app) printGeneration(g api.Generation) {
	what := string(g.Kind)
	if g.Inputs != nil && g.Inputs.Mode != nil {
		what += " (" + string(*g.Inputs.Mode) + ")"
	}
	a.printf("%s  %s  %s  %s\n", stamp(g.CreatedAt), what, g.Status, g.RequestId)
	by := "unknown maker"
	if g.MadeBy != nil {
		by = makerName(*g.MadeBy)
	}
	charge := "charge unknown"
	if g.MicrocreditsCharged != nil {
		charge = credits(int64(*g.MicrocreditsCharged)) + " credits"
	}
	a.printf("  by %s · %s\n", by, charge)
	if text := generationText(g.Inputs); text != "" {
		a.printf("  %q\n", text)
	}
	for _, o := range g.Outputs {
		switch o.Status {
		case avatarapi.GenerationOutputStatus("ready"):
			a.printf("  upload:%s  until %s\n", o.UploadId, stamp(o.ExpiresAt))
			if o.Url != nil {
				a.printf("    %s\n", *o.Url)
			}
		default:
			a.printf("  upload:%s  %s\n", o.UploadId, o.Status)
		}
	}
}

// makerName is who made a generation, as the list and the summary name them.
func makerName(m avatarapi.GenerationMaker) string {
	name := "(no name)"
	if m.Name != nil && *m.Name != "" {
		name = *m.Name
	}
	if m.Type == avatarapi.GenerationMakerType("apiKey") {
		name = "API key " + name
		if m.Revoked != nil && *m.Revoked {
			name += " (revoked)"
		}
	}
	return name
}

// generationText is the words a generation was made from (a clip's
// transcript or prompt, a portrait's or voice's description), cut short.
func generationText(in *avatarapi.GenerationInputs) string {
	if in == nil {
		return ""
	}
	for _, s := range []*string{in.Transcript, in.Prompt, in.Description, in.VoiceDescription} {
		if s != nil && strings.TrimSpace(*s) != "" {
			text := strings.Join(strings.Fields(*s), " ")
			if r := []rune(text); len(r) > 80 {
				text = string(r[:79]) + "…"
			}
			return text
		}
	}
	return ""
}

func (a *app) generateSummaryCmd() *cobra.Command {
	var h historyFlags
	var jsonOut bool
	c := &cobra.Command{
		Use:   "summary",
		Short: "Count the account's generations by kind, and list who made them.",
		Long: "Count the account's generations by kind and its clips by mode, under the same --made-by, --status, --since and --query " +
			"filters as `atmee generate list`, and list everyone who made any of them (whatever the filters), latest first, with the id " +
			"--made-by takes. " + historyNote,
		Example: `  atmee generate summary
  atmee generate summary --status ready --since 30d`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := h.filter(time.Now())
			if err != nil {
				return err
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			s, err := cl.GenerationSummary(ctx(), f)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(s)
			}
			a.printf("%s: %s (%d picture, %d avatar, %d unknown), %s, %s\n", plural(s.Total, "generation"),
				plural(s.Counts.Clip, "clip"), s.ClipModes.Picture, s.ClipModes.Avatar, s.ClipModes.Unknown,
				plural(s.Counts.Portrait, "portrait call"), plural(s.Counts.Voice, "voice call"))
			if len(s.Makers) > 0 {
				a.printf("Made by:\n")
				for _, m := range s.Makers {
					id := ""
					switch {
					case m.MadeBy.UserId != nil:
						id = m.MadeBy.UserId.String()
					case m.MadeBy.ApiKeyId != nil:
						id = m.MadeBy.ApiKeyId.String()
					}
					a.printf("  %s  %s  %s, last %s\n", makerName(m.MadeBy), id, plural(m.Generations, "generation"), stamp(m.LastAt))
				}
			}
			if s.UnknownGenerations > 0 {
				a.printf("%s with no recorded maker (older calls)\n", plural(s.UnknownGenerations, "generation"))
			}
			return nil
		},
	}
	h.add(c)
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) generateDeleteCmd() *cobra.Command {
	var operation string
	var confirm bool
	c := &cobra.Command{
		Use:   "delete REQUEST_ID",
		Short: "Delete a generation's outputs (asks first; --yes skips the question).",
		Long: "Delete the outputs of the generation made under REQUEST_ID (`atmee generate list` prints it): their files go at once and " +
			"their links stop working; the list shows it as deleted, with its inputs and poster kept, so it can be made again. Nothing is " +
			"refunded. Deleting again is no error. A generation still rendering answers 409 generation_rendering: delete it once it has " +
			"finished. If the request id was used for more than one operation, pass --operation (the list's --json shows it), or the delete " +
			"answers 409 generation_ambiguous and deletes nothing. Any member may delete any of the account's generations; an API key " +
			"scoped to an avatar only its avatar's. A signed-in user, or an API key with generation:write or avatars:write.",
		Example: `  atmee generate delete 5b0c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f
  atmee generate delete 5b0c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f --operation clip --yes`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := api.ParseGenerationRequestID(args[0]); err != nil {
				return usageError{err}
			}
			if operation != "" && !slices.Contains(api.GenerationOperations, operation) {
				return usagef("--operation takes %s (%s)", strings.Join(api.GenerationOperations, ", "), operation)
			}
			if !confirm {
				if err := a.confirm(fmt.Sprintf("Delete the outputs of generation %s? Nothing is refunded. [y/N]", args[0])); err != nil {
					return err
				}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			if err := cl.DeleteGeneration(ctx(), args[0], operation); err != nil {
				return err
			}
			a.printf("Deleted the outputs of %s.\n", args[0])
			return nil
		},
	}
	c.Flags().StringVar(&operation, "operation", "", "The generation's operation, needed only when its request id was used for several: "+strings.Join(api.GenerationOperations, ", "))
	c.Flags().BoolVar(&confirm, "yes", false, "Delete without asking")
	return c
}
