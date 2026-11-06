package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

func (a *app) avatarsCmd() *cobra.Command {
	c := group("avatars", "Create, build, pick and talk to your avatars", a.createCmd(), a.listCmd(), a.getCmd(), a.waitCmd(), a.updateCmd(), a.deleteCmd(),
		a.deleteVersionCmd(),
		a.replaceCmd("replace-voice", "Replace the voice sample (a local file, an https URL or upload:ID). v2: builds a new candidate; the current one keeps serving until it is finished (with manual selection, until you pick a new one).", (*api.Client).ReplaceVoice),
		a.replaceCmd("replace-appearance", "Replace the portrait (a local file, an https URL or upload:ID: front-facing, face at least a third of the frame, lips slightly parted). v2: builds new candidates from it.", (*api.Client).ReplaceAppearance),
		a.replaceKnowledgeCmd(), a.getKnowledgeCmd(), a.questionnaireCmd(), a.thumbnailCmd(), a.monetizationCmd(), a.getFileCmd("get-appearance"), a.getFileCmd("get-voice"), a.candidatesCmd(), a.conductorCmd(), a.sponsorCmd(), a.sessionCmd())
	// The local mock's failure injection on any avatar command (X-Mock-Scenario).
	c.PersistentFlags().StringVar(&a.mockScenario, "mock-scenario", "", "Local mock only")
	_ = c.PersistentFlags().MarkHidden("mock-scenario")
	return c
}

// configFile is a --config JSON object: the create flags in snake_case.
type configFile struct {
	Image            *string        `json:"image"`
	Voice            *string        `json:"voice"`
	Engine           *string        `json:"engine"`
	SeedSelection    *string        `json:"seed_selection"`
	SeedPreviews     *int           `json:"seed_previews"`
	PreviewMessage   *string        `json:"preview_message"`
	Description      *string        `json:"description"`
	Language         *string        `json:"language"`
	Visibility       *string        `json:"visibility"`
	Previews         *bool          `json:"previews"`
	Facts            *string        `json:"facts"`
	Knowledge        []string       `json:"knowledge"`
	WhoPrompt        *string        `json:"who_prompt"`
	WhatPrompt       *string        `json:"what_prompt"`
	Personality      *string        `json:"personality"` // older name of who_prompt
	Scenario         *string        `json:"scenario"`    // older name of what_prompt
	Greeting         *string        `json:"greeting"`
	SelfIntroduction *string        `json:"self_introduction"`
	Guardrails       []string       `json:"guardrails"`
	PreferredAlias   *string        `json:"preferred_alias"`
	SpokenName       *string        `json:"spoken_name"`
	KnownAliases     any            `json:"known_aliases"`
	Habits           map[string]any `json:"conversational_habits"`
	SponsorMicro     *int           `json:"sponsor_microcredits"`
	SponsorCap       *int           `json:"sponsor_max_per_session"`
	RuntimeSeed      *int           `json:"runtime_seed"`
	Resolution       *configRes     `json:"resolution"`
	Name             *string        `json:"name"` // ignored: --name wins
}

// configRes is --config's resolution: "WxH" as --resolution takes it, or
// {"width": W, "height": H} as the API does.
type configRes api.Resolution

func (r *configRes) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		v, err := api.ParseResolution(s)
		if err != nil {
			return err
		}
		*r = configRes(v)
		return nil
	}
	var o struct {
		Width  *int `json:"width"`
		Height *int `json:"height"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&o); err != nil || o.Width == nil || o.Height == nil {
		return errors.New(`resolution takes "WxH" (e.g. "672x384") or {"width": W, "height": H}`)
	}
	*r = configRes{Width: *o.Width, Height: *o.Height}
	return nil
}

func readConfig(path string, in io.Reader) (configFile, error) {
	var cfg configFile
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(in)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return cfg, usagef("could not read the input file: %v", err)
	}
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return cfg, usagef("could not read the input file: %v", err)
	}
	if _, ok := probe.(map[string]any); !ok {
		return cfg, usagef("--config must hold a JSON object")
	}
	// Unknown fields fail: a typo must not silently create an avatar with
	// defaults (and start captures).
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, usagef("--config: %v", err)
	}
	return cfg, nil
}

func (a *app) createCmd() *cobra.Command {
	var (
		name, image, voice, eng, selection, previewMsg, whoPrompt, whatPrompt, greeting, selfIntro string
		facts, factsFile, alias, spokenName, language, visibility, configPath, resolution          string
		candidates, sponsorMicro, sponsorCap, runtimeSeed                                          int
		guardrails, knowledge, knownAliases, habits                                                []string
		previews, noPreviews, wait, jsonOut                                                        bool
	)
	c := &cobra.Command{
		Use:   "create",
		Short: "Create an avatar from a portrait and a voice sample, now or later.",
		Long: "Create an avatar. The default engine, v2, builds from --image (a portrait) and --voice (10-60 s of one " +
			"person speaking, which is cloned): one candidate, a short talking clip, used as soon as it is finished. If " +
			"its lips don't match, build more (`atmee avatars candidates add`) and choose one " +
			"(`atmee avatars candidates pick` or `select`).\n\n" +
			"Atmee V2 HD (--engine v2hd), a larger v2 model at larger frame sizes (640x640 by default), is for " +
			"Enterprise customers on custom plans: contact us through Creator Studio's help center. Existing V2 HD " +
			"avatars keep working.\n\n" +
			"Both files can come later: without one the avatar is a draft (status draft) whose persona and knowledge " +
			"build; give them with `atmee avatars replace-appearance` and `replace-voice`, in either order, and it " +
			"builds then. A v1 avatar with only --image is render-only (no voice).\n\n" +
			"Portrait: a front-facing photo with the whole head and some margin, the mouth slightly open as if " +
			"mid-sentence. A closed mouth can give candidates whose mouth never moves.",
		Example: `  atmee avatars create --name Ada --image ./portrait.jpg --voice ./voice.m4a --wait
  atmee avatars create --name Ada --image https://example.com/ada.jpg --voice https://example.com/ada.m4a \
    --who-prompt "A patient chess coach" --greeting "Hi, ready for a game?"
  atmee avatars create --name Ada --config avatar.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := api.CreateInput{Name: name}
			persona := map[string]any{}
			if configPath != "" {
				cfg, err := readConfig(configPath, a.in)
				if err != nil {
					return err
				}
				in.Image, in.Voice, in.Engine = deref(cfg.Image), deref(cfg.Voice), deref(cfg.Engine)
				in.SeedSelection, in.Candidates, in.PreviewMessage = deref(cfg.SeedSelection), deref(cfg.SeedPreviews), deref(cfg.PreviewMessage)
				in.Description, in.Language, in.Visibility = deref(cfg.Description), deref(cfg.Language), deref(cfg.Visibility)
				in.Previews, in.Facts, in.Knowledge = cfg.Previews, deref(cfg.Facts), cfg.Knowledge
				in.SponsorMicrocredits, in.SponsorMaxPerSession = cfg.SponsorMicro, cfg.SponsorCap
				in.RuntimeSeed = cfg.RuntimeSeed
				if cfg.Resolution != nil {
					r := api.Resolution(*cfg.Resolution)
					in.Resolution = &r
				}
				if cfg.WhoPrompt != nil && cfg.Personality != nil {
					return usagef("--config: who_prompt and personality are the same field (personality is its older name): give one")
				}
				if cfg.WhatPrompt != nil && cfg.Scenario != nil {
					return usagef("--config: what_prompt and scenario are the same field (scenario is its older name): give one")
				}
				if cfg.WhoPrompt == nil {
					cfg.WhoPrompt = cfg.Personality
				}
				if cfg.WhatPrompt == nil {
					cfg.WhatPrompt = cfg.Scenario
				}
				for k, v := range map[string]*string{"whoPrompt": cfg.WhoPrompt, "whatPrompt": cfg.WhatPrompt, "greeting": cfg.Greeting,
					"selfIntroduction": cfg.SelfIntroduction, "preferredAlias": cfg.PreferredAlias, "spokenName": cfg.SpokenName} {
					if v != nil {
						persona[k] = *v
					}
				}
				if cfg.Guardrails != nil {
					persona["guardrails"] = cfg.Guardrails
				}
				if cfg.KnownAliases != nil {
					persona["knownAliases"] = splitAliases(cfg.KnownAliases)
				}
				if cfg.Habits != nil {
					persona["conversationalHabits"] = cfg.Habits
				}
			}
			f := cmd.Flags()
			setStr := func(flag string, dst *string, v string) {
				if f.Changed(flag) {
					*dst = v
				}
			}
			setStr("image", &in.Image, image)
			setStr("voice", &in.Voice, voice)
			setStr("engine", &in.Engine, eng)
			setStr("seed-selection", &in.SeedSelection, selection)
			setStr("preview-message", &in.PreviewMessage, previewMsg)
			setStr("language", &in.Language, language)
			setStr("visibility", &in.Visibility, visibility)
			setStr("facts", &in.Facts, facts)
			if f.Changed("candidates") {
				in.Candidates = candidates
			}
			if f.Changed("sponsor-microcredits") {
				in.SponsorMicrocredits = &sponsorMicro
			}
			if f.Changed("sponsor-max-per-session") {
				in.SponsorMaxPerSession = &sponsorCap
			}
			if f.Changed("runtime-seed") {
				in.RuntimeSeed = &runtimeSeed
			}
			if f.Changed("resolution") {
				r, err := api.ParseResolution(resolution)
				if err != nil {
					return usagef("%v", err)
				}
				in.Resolution = &r
			}
			if factsFile != "" {
				b, err := os.ReadFile(factsFile)
				if err != nil {
					return usagef("could not read the input file: %v", err)
				}
				in.Facts = string(b)
			}
			if f.Changed("knowledge") {
				in.Knowledge = knowledge
			}
			if previews {
				t := true
				in.Previews = &t
			} else if noPreviews {
				t := false
				in.Previews = &t
			}
			for flag, key := range map[string]string{"who-prompt": "whoPrompt", "what-prompt": "whatPrompt", "greeting": "greeting",
				"self-intro": "selfIntroduction", "alias": "preferredAlias", "spoken-name": "spokenName"} {
				if f.Changed(flag) {
					v, _ := f.GetString(flag)
					persona[key] = v
				}
			}
			if f.Changed("guardrail") {
				persona["guardrails"] = guardrails
			}
			if f.Changed("known-alias") {
				if err := checkKnownAliases(knownAliases); err != nil {
					return err
				}
				persona["knownAliases"] = knownAliases
			}
			if f.Changed("habit") {
				h, err := parseHabits(habits, nil)
				if err != nil {
					return err
				}
				persona["conversationalHabits"] = h
			}
			if len(persona) > 0 {
				in.Persona = persona
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			// v2 is the default, with one candidate used as soon as it is
			// finished, wherever the flags and --config chose nothing. An
			// account that cannot build v2 (Atmee V2 not on its plan or not
			// in where v2 is still invite-only, or v2 not on this server)
			// gets v1 instead, unless v2-only options were asked for. A draft
			// (no portrait or no voice yet) gets the candidate options from
			// the API when it is completed.
			defaulted := in.Engine == ""
			v2Options := in.Candidates != 0 || in.SeedSelection != "" || in.Resolution != nil
			userCandidates, userSelection := in.Candidates, in.SeedSelection
			if defaulted {
				in.Engine = api.EngineV2
				if why := noV2Reason(cl); why != "" {
					if v2Options {
						return fmt.Errorf("%s; --candidates, --seed-selection and --resolution are for v2 avatars: drop them to make a v1 avatar", why)
					}
					in.Engine = api.EngineV1
					a.note("Making a v1 avatar: %s.\n", why)
				}
			}
			complete := in.Image != "" && in.Voice != ""
			if api.IsV2Family(in.Engine) && complete && in.SeedSelection == "" {
				in.SeedSelection = "auto"
			}
			if api.IsV2Family(in.Engine) && complete && in.Candidates == 0 {
				in.Candidates = 1
			}
			if r := in.Resolution; r != nil {
				if err := cl.CheckSize(ctx(), in.Engine, *r); err != nil {
					var pro *api.V2ProSizeError
					if errors.As(err, &pro) {
						return usagef("%v: that size is Atmee V2 HD's, and V2 HD is Enterprise-only (contact us through Creator Studio's help center); pick a v2 size (`atmee account` lists them)", err)
					}
					return usagef("%v", err)
				}
			}
			res, quota, err := cl.Create(ctx(), in)
			// A v2 create the CLI chose, refused for the account before
			// anything was made: make it v1 instead.
			if err != nil && defaulted && !v2Options && in.Engine == api.EngineV2 && v2Refused(err) {
				in.Engine, in.Candidates, in.SeedSelection = api.EngineV1, userCandidates, userSelection
				a.note("Making a v1 avatar: %v.\n", err)
				res, quota, err = cl.Create(ctx(), in)
			}
			if err != nil {
				return err
			}
			if !wait {
				if jsonOut {
					return a.printJSON(res)
				}
				a.printf("Created %s (%s, %s)\n", res.AvatarId, engine(deref(res.Engine)), res.Status)
				if res.MissingInputs != nil {
					a.noteMissing(res.AvatarId.String(), missingNames(*res.MissingInputs))
				}
				a.reportSponsor(res.Sponsor)
				if quota != nil {
					a.note("Room for %d more captures\n", quota.Remaining)
				}
				return nil
			}
			// --wait prints the settled avatar; the funding outcome is not
			// on it, so say it first (stderr keeps --json clean).
			a.reportSponsor(res.Sponsor)
			return a.waitAndPrint(cl, res.AvatarId.String(), 15*time.Minute, jsonOut)
		},
	}
	f := c.Flags()
	f.StringVar(&name, "name", "", "Avatar name (required)")
	_ = c.MarkFlagRequired("name")
	f.StringVar(&image, "image", "", "Portrait: local file, https URL or upload:ID (from atmee generate). Front-facing, face at least a third of the frame, lips slightly parted (without it: a draft, completed by replace-appearance)")
	f.StringVar(&voice, "voice", "", "Voice sample to clone, 10-60 s: local file, https URL or upload:ID (from atmee generate) (without it: a draft, completed by replace-voice; a v1 avatar with only --image is render-only)")
	f.StringVar(&eng, "engine", "", "v2 (default), v1, or v2hd (Atmee V2 HD, for Enterprise customers on custom plans; v2pro, its first name, also works)")
	f.IntVar(&candidates, "candidates", 0, "v2, with --image and --voice: how many candidates to build, 1-10 (default 1)")
	f.StringVar(&selection, "seed-selection", "", "v2, with --image and --voice: auto (default: the first one finished is used) or manual (you pick a candidate)")
	f.StringVar(&previewMsg, "preview-message", "", "The line the candidates speak (v2: at most 300 characters)")
	f.StringVar(&whoPrompt, "who-prompt", "", whoPromptHelp)
	f.StringVar(&whatPrompt, "what-prompt", "", whatPromptHelp)
	f.StringVar(&greeting, "greeting", "", "The first thing it says in a conversation")
	f.StringVar(&selfIntro, "self-intro", "", "How it introduces itself (more than 50 characters; derived from --who-prompt when omitted)")
	f.StringArrayVar(&guardrails, "guardrail", nil, "A rule it must follow (repeat for several)")
	f.StringVar(&facts, "facts", "", factsHelp)
	f.StringVar(&factsFile, "facts-file", "", "Read --facts from a text file")
	f.StringArrayVar(&knowledge, "knowledge", nil, "A knowledge file, PDF, DOCX, TXT, ... (repeat for several; "+knowledgeNote+")")
	f.StringVar(&alias, "alias", "", "The short name it goes by in conversation")
	f.StringVar(&spokenName, "spoken-name", "", spokenNameHelp)
	f.StringArrayVar(&knownAliases, "known-alias", nil, knownAliasHelp+" (repeat for several)")
	f.StringArrayVar(&habits, "habit", nil, habitHelp)
	f.StringVar(&language, "language", "", "Language it speaks: en, zh-TW, ja, ...")
	f.StringVar(&visibility, "visibility", "", "private (default) or unlisted (public: set the Explore profile and a thumbnail, then `avatars update --visibility public`)")
	f.BoolVar(&previews, "previews", false, "v1: also render preview clips")
	f.BoolVar(&noPreviews, "no-previews", false, "v1: no preview clips")
	f.StringVar(&configPath, "config", "", "Read these options from a JSON file, keys in snake_case (- reads stdin)")
	f.BoolVar(&wait, "wait", false, "Wait until it is ready (with --seed-selection manual, until you need to pick a candidate)")
	f.IntVar(&sponsorMicro, "sponsor-microcredits", 0, "Put credits in its sponsor pool so visitors talk for free (100 = 1 credit; a signed-in user, or an API key with billing:write)")
	f.IntVar(&sponsorCap, "sponsor-max-per-session", 0, "Most the sponsor pool pays for one conversation, in microcredits")
	f.IntVar(&runtimeSeed, "runtime-seed", 0, runtimeSeedHelp)
	f.StringVar(&resolution, "resolution", "", resolutionHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	promptAliases(c)
	return c
}

// whoPromptHelp and whatPromptHelp describe Creator Studio's two persona
// prompts, "Who are you" and "What are we doing".
const (
	whoPromptHelp  = "Who the avatar is (Studio's \"Who are you\"): character, background, tone"
	whatPromptHelp = "What it does with the people it talks to (Studio's \"What are we doing\")"
)

// promptAliases keeps the older names of --who-prompt and --what-prompt
// working, hidden: --personality and --who, --scenario and --what.
func promptAliases(c *cobra.Command) {
	for _, p := range [][2]string{{"personality", "who-prompt"}, {"who", "who-prompt"}, {"scenario", "what-prompt"}, {"what", "what-prompt"}} {
		alias, target := p[0], p[1]
		c.Flags().String(alias, "", "Alias of --"+target)
		_ = c.Flags().MarkHidden(alias)
		aliasFlag(c, alias, target)
	}
}

// aliasFlag makes --alias set the same value as --target and count as it.
func aliasFlag(c *cobra.Command, alias, target string) {
	prev := c.PreRunE
	c.PreRunE = func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		if f.Changed(alias) && !f.Changed(target) {
			v, _ := f.GetString(alias)
			if err := f.Set(target, v); err != nil {
				return err
			}
		}
		if prev != nil {
			return prev(cmd, args)
		}
		return nil
	}
}

func splitAliases(v any) any {
	if s, ok := v.(string); ok {
		var out []string
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return v
}

func (a *app) waitAndPrint(cl *api.Client, id string, timeout time.Duration, jsonOut bool) error {
	var onUpdate func(*api.Avatar)
	if !jsonOut {
		onUpdate = a.progress
	}
	av, err := cl.Wait(ctx(), id, timeout, pollInterval, onUpdate)
	if av != nil {
		if jsonOut {
			// Printed even when the wait failed, so a script sees the state;
			// a failed write is the error when nothing else went wrong.
			if perr := a.printJSON(av); perr != nil && err == nil {
				err = perr
			}
		} else if err == nil {
			a.printAvatar(av)
		}
	}
	return err
}

func (a *app) listCmd() *cobra.Command {
	var limit int
	var before string
	var all, jsonOut bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List your avatars, newest first.",
		Long:  "List your avatars, newest first. One page at a time: --all fetches every page, --before continues from the nextBefore a page printed.",
		Example: `  atmee avatars list
  atmee avatars list --all --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var cursor *time.Time
			if before != "" {
				t, err := time.Parse(time.RFC3339Nano, before)
				if err != nil {
					return usagef("--before takes the nextBefore timestamp of a previous page: %v", err)
				}
				cursor = &t
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			var items []api.AvatarSummary
			for {
				page, next, err := cl.List(ctx(), limit, cursor)
				if err != nil {
					return err
				}
				items = append(items, page...)
				cursor = next
				if !all || next == nil || len(page) == 0 {
					break
				}
			}
			if jsonOut {
				return a.printJSON(map[string]any{"items": items, "nextBefore": cursor})
			}
			a.printAvatars(items)
			if cursor != nil {
				a.note("More avatars: atmee avatars list --before %s (or --all)\n", cursor.Format(time.RFC3339Nano))
			}
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 50, "Avatars per page")
	c.Flags().StringVar(&before, "before", "", "Start after this point: the nextBefore the previous page printed")
	c.Flags().BoolVar(&all, "all", false, "List every page")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) getCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "get AVATAR_ID",
		Short: "Show an avatar: its status, build progress and what failed.",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			av, err := cl.Get(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(av)
			}
			a.printAvatar(av)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) waitCmd() *cobra.Command {
	var timeout int
	var jsonOut bool
	c := &cobra.Command{
		Use:   "wait AVATAR_ID",
		Short: "Wait until an avatar is ready, or until you need to pick a candidate.",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			return a.waitAndPrint(cl, args[0], time.Duration(timeout)*time.Second, jsonOut)
		},
	}
	c.Flags().IntVar(&timeout, "timeout", 900, "Give up after this many seconds")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// factsHelp and knowledgeNote say where the knowledge goes and who uses it.
const (
	factsHelp = "What it knows, as text (up to 32,000 characters): saved as Studio's \"Facts with context\" and read by " +
		"the knowledge base (" + knowledgeNote + ")"
	knowledgeNote = "v1 conversations use knowledge; v2 ones do not yet"
)

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (a *app) deleteCmd() *cobra.Command {
	var confirm bool
	c := &cobra.Command{
		Use:   "delete AVATAR_ID",
		Short: "Delete an avatar (asks first; --yes skips the question).",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				if err := a.confirm(fmt.Sprintf("Delete avatar %s? [y/N]", args[0])); err != nil {
					return err
				}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			if err := cl.Delete(ctx(), args[0]); err != nil {
				return err
			}
			a.printf("Deleted %s.\n", args[0])
			return nil
		},
	}
	c.Flags().BoolVar(&confirm, "yes", false, "Delete without asking")
	c.Flags().BoolVar(&confirm, "confirm", false, "Same as --yes")
	_ = c.Flags().MarkHidden("confirm")
	return c
}

func (a *app) deleteVersionCmd() *cobra.Command {
	var confirm, jsonOut bool
	c := &cobra.Command{
		Use:   "delete-version AVATAR_ID VERSION_ID",
		Short: "Delete an older version of a v2 avatar, for good (asks first; --yes skips the question).",
		Long: "Delete every take of an older version of a v2 avatar and the files only that version uses; it cannot be restored. " +
			"The current version, and one holding the serving or selected take, answer 409 version_in_use; one whose capture is still " +
			"running answers 409 version_busy. If storage could not delete everything, the answer counts what is left: run it again.",
		Example: `  atmee avatars candidates list AVATAR_ID --all
  atmee avatars delete-version AVATAR_ID 1a2b3c4d`,
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := api.ParseID(args[0]); err != nil {
				return usagef("%v", err)
			}
			if !versionID.MatchString(args[1]) {
				return usagef("%q is not a version id (8 hex characters; find it with `atmee avatars candidates list %s --all`)", args[1], args[0])
			}
			if !confirm {
				if err := a.confirm(fmt.Sprintf("Delete version %s of avatar %s and its takes for good? [y/N]", args[1], args[0])); err != nil {
					return err
				}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			d, err := cl.DeleteVersion(ctx(), args[0], args[1])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(d)
			}
			a.printf("Deleted version %s: %s, %s.\n", d.VersionId, plural(len(d.DeletedCandidateIds), "take"), plural(d.DeletedFiles, "file"))
			if d.PendingFiles > 0 {
				a.printf("%s could not be deleted yet: run it again to retry.\n", plural(d.PendingFiles, "file"))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&confirm, "yes", false, "Delete without asking")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// versionID is a version id's shape (see the API's versionId parameter).
var versionID = regexp.MustCompile(`^[0-9a-f]{8}$`)

func (a *app) replaceCmd(use, short string, call func(*api.Client, context.Context, string, string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " AVATAR_ID PATH",
		Short: short,
		Args:  exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			if err := call(cl, ctx(), args[0], args[1]); err != nil {
				return err
			}
			a.printf("Rebuilding.\n")
			return nil
		},
	}
}

func (a *app) replaceKnowledgeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "replace-knowledge AVATAR_ID [FILE...]",
		Short: "Replace the knowledge files (PDF, DOCX, TXT, MD, ...); with no files, remove them all. The --facts text keeps its own file (change it with `update --facts`).",
		Args:  minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			if err := cl.ReplaceKnowledge(ctx(), args[0], args[1:]); err != nil {
				return err
			}
			if len(args) > 1 {
				a.printf("Knowledge updated.\n")
			} else {
				a.printf("Knowledge cleared.\n")
			}
			return nil
		},
	}
}

func (a *app) getKnowledgeCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "get-knowledge AVATAR_ID",
		Short: "Show what the avatar knows: the --facts text, the questionnaire answers and the knowledge files (with download links valid for an hour).",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			k, err := cl.Knowledge(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(k)
			}
			a.printKnowledge(k)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// getFileCmd is get-appearance or get-voice: the file last given, as a
// download link, or saved with --out.
func (a *app) getFileCmd(use string) *cobra.Command {
	var (
		jsonOut bool
		out     string
	)
	what := map[string]string{"get-appearance": "portrait", "get-voice": "voice sample"}[use]
	c := &cobra.Command{
		Use:   use + " AVATAR_ID",
		Short: "Show a download link (valid for an hour) of the " + what + " last given, or save it with --out.",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			var (
				res     any
				link    *string
				expires *time.Time
			)
			if use == "get-voice" {
				v, err := cl.Voice(ctx(), args[0])
				if err != nil {
					return err
				}
				// ttsProvider is deprecated and always "": every avatar
				// speaks with a Fish Audio voice cloned from its sample.
				res, link, expires = v, v.Url, v.ExpiresAt
			} else {
				f, err := cl.Appearance(ctx(), args[0])
				if err != nil {
					return err
				}
				res, link, expires = f, f.Url, f.ExpiresAt
			}
			// A null link is no file given, or one that could not be signed
			// just now.
			noLink := "No download link: the avatar has no " + what + ", or it could not be signed just now (try again)."
			if out != "" {
				if link == nil {
					return errors.New(strings.ToLower(noLink[:1]) + strings.TrimSuffix(noLink[1:], "."))
				}
				if err := download(*link, out); err != nil {
					return err
				}
				a.note("Saved the %s to %s\n", what, out)
			}
			if jsonOut {
				return a.printJSON(res)
			}
			switch {
			case out != "":
			case link == nil:
				a.printf("%s\n", noLink)
			default:
				a.printf("%s\n", *link)
				if expires != nil {
					a.note("Valid until %s\n", expires.Local().Format(time.RFC1123))
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	c.Flags().StringVar(&out, "out", "", "Save the file to this path instead of printing the link")
	return c
}

// reportSponsor says whether an on-create grant funded the pool; a refused
// one (insufficient balance, free-plan credits) leaves it empty.
func (a *app) reportSponsor(sp *api.SponsorGrant) {
	if sp == nil {
		return
	}
	if sp.Error != nil {
		a.note("Sponsor pool not funded: %s (fund it with `atmee avatars sponsor grant`)\n", *sp.Error)
		return
	}
	a.note("Sponsor pool funded with %d microcredits\n", sp.TransferredMicrocredits)
}

// runtimeSeedHelp is the --runtime-seed flag's help on create and update.
const runtimeSeedHelp = "v2: the renderer's runtime seed, 0-2147483647 (default 42): another seed gives different motion " +
	"during conversations; the take and its preview stay the same. Applies from the next session; nothing rebuilds"

// resolutionHelp is the --resolution flag's help on create and update.
const resolutionHelp = "v2: the frame size as `WxH`, one the server offers: 512x512 (1:1, the default), 672x384 (16:9), " +
	"384x672 (9:16), 576x448 (4:3) or 448x576 (3:4). V2 HD: 640x640 (1:1, the default), 864x480 (16:9), 480x864 (9:16), " +
	"736x544 (4:3) or 544x736 (3:4). A size not offered is refused with the list (`atmee account` lists what the server " +
	"offers for both). The portrait is framed at that size, so a new size builds new candidates"

// spokenNameHelp and knownAliasHelp describe the persona's name flags
// (Studio's "Pronunciation" and "Known aliases").
const (
	spokenNameHelp = "How its name is pronounced, when that differs from how it is written"
	knownAliasHelp = "Another name people may call it, without commas"
)

// habitNames are Studio's conversational style sliders, by their flag name.
var habitNames = []string{"engagement", "wordiness", "delivery", "inquiry", "focus"}

const habitHelp = "A conversational style slider as NAME=LEVEL: engagement, wordiness, delivery, inquiry or focus = low, medium or high (repeat for several)"

// parseHabits applies NAME=LEVEL settings over current (the avatar's
// sliders, nil on create) and returns the API's conversationalHabits.
func parseHabits(settings []string, current map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range current {
		out[k] = v
	}
	for _, s := range settings {
		name, level, ok := strings.Cut(strings.ToLower(s), "=")
		name, level = strings.TrimSpace(name), strings.TrimSpace(level)
		if !ok || !slices.Contains(habitNames, name) || (level != "low" && level != "medium" && level != "high") {
			return nil, usagef("--habit %q: use NAME=LEVEL, NAME one of %s and LEVEL low, medium or high", s, strings.Join(habitNames, ", "))
		}
		out["CONVERSATIONAL_HABIT_"+strings.ToUpper(name)] = "CONVERSATIONAL_HABIT_SCALE_" + strings.ToUpper(level)
	}
	return out, nil
}

// checkKnownAliases refuses what the API would refuse: the aliases are
// stored as comma-separated text, so one alias may not contain a comma.
func checkKnownAliases(aliases []string) error {
	for _, a := range aliases {
		if strings.TrimSpace(a) == "" || strings.Contains(a, ",") {
			return usagef("--known-alias %q: one name, not empty and without commas", a)
		}
	}
	return nil
}

// jsonHelp is the --json flag's help everywhere.
const jsonHelp = "Print the API's JSON instead of a summary"

// downloadTimeout bounds one file download (portraits and voice samples
// are at most tens of MB).
const downloadTimeout = 2 * time.Minute

// download saves the file at a presigned link to path.
func download(link, path string) error {
	res, err := (&http.Client{Timeout: downloadTimeout}).Get(link)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", res.Status)
	}
	// Written beside path and moved over it once complete, so a failed
	// transfer leaves an existing file as it was.
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

const redeemV2Hint = "redeem an invite code with `atmee redeem CODE` (or in Creator Studio's Billing page)"

// noV2Plan is the refusal of Atmee V2 to an account whose plan doesn't
// include it and that no invite code unlocked, and getV2Hint how to get it.
const (
	noV2Plan  = "Atmee V2 isn't on this account's plan"
	getV2Hint = "upgrade to Growth and up, or `atmee redeem CODE` with an invite code"
)

// engineOffer says where `atmee account` shows which engines a server offers.
const engineOffer = "`atmee account` shows the v2 avatars line, and a V2 HD line only where V2 HD is offered"

// engineHints are what to do about an engine or plan refusal, by its code:
// on a create, which takes --engine, and on any other command (an update,
// a capture, a session), where the avatar's engine is already set.
var engineHints = map[string]struct{ create, other string }{
	"v2_pro_not_in_plan": {
		create: "Atmee V2 HD is for Enterprise customers on custom plans: contact us through Creator Studio's help center, or use --engine v2",
		other:  "Atmee V2 HD is for Enterprise customers on custom plans: contact us through Creator Studio's help center, or use a v2 avatar",
	},
	"v2_access_required": {
		create: "an invite code hasn't unlocked Atmee V2 on this account yet (V2 avatars and video clips need it here): " + redeemV2Hint + ", or use --engine v1",
		other:  "an invite code hasn't unlocked Atmee V2 on this account yet (V2 avatars and video clips need it here): " + redeemV2Hint,
	},
	"insufficient_v2_credits": {
		create: noV2Plan + ": " + getV2Hint + " (or use --engine v1)",
		other:  noV2Plan + ": " + getV2Hint,
	},
	"engine_unavailable": {
		create: "this server does not offer that engine (" + engineOffer + "): use another --engine",
		other:  "this server does not offer this avatar's engine right now (" + engineOffer + "): use an avatar on an engine it offers, or another server",
	},
}

// engineHint is what to do about an engine or plan refusal on cmd, or ""
// for any other error.
func engineHint(cmd *cobra.Command, err error) string {
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		return ""
	}
	h, ok := engineHints[apiErr.Code]
	if !ok {
		return ""
	}
	if cmd != nil && cmd.Flags().Lookup("engine") != nil {
		return h.create
	}
	return h.other
}

// noV2Reason is why the account cannot make v2 avatars, or "" when it can
// or that can't be told (the create then says so itself). A reported
// balance usable on Atmee V2 decides for every account: above 0 it can
// build v2, 0 it can't. v2Access decides only where the balance leaves the
// figure out (billing still deciding v2 by invite). A balance that can't be
// read decides nothing.
func noV2Reason(cl *api.Client) string {
	acct, err := cl.Account(ctx())
	if err != nil {
		return ""
	}
	if acct.V2Access != nil && string(*acct.V2Access) == api.V2AccessUnavailable {
		return "v2 avatars are not offered on this server"
	}
	noV2 := noV2Plan + " (Growth and up), and no invite code has unlocked it; to make V2, " + getV2Hint
	b, err := cl.Balance(ctx())
	if err != nil {
		// The balance could not be read: let the create decide rather
		// than quietly make v1 for an account that may have credits.
		return ""
	}
	if b.V2SpendableMicrocredits != nil {
		if *b.V2SpendableMicrocredits > 0 {
			return ""
		}
		return noV2
	}
	if acct.V2Access != nil && string(*acct.V2Access) == api.V2AccessInviteRequired {
		return noV2
	}
	return ""
}

// v2Refused says the API refused v2 to the account: not unlocked (403
// v2_access_required) or nothing usable on Atmee V2 (402
// insufficient_v2_credits).
func v2Refused(err error) bool {
	var apiErr *api.Error
	return errors.As(err, &apiErr) && ((apiErr.Status == http.StatusForbidden && apiErr.Code == "v2_access_required") ||
		(apiErr.Status == http.StatusPaymentRequired && apiErr.Code == "insufficient_v2_credits"))
}
