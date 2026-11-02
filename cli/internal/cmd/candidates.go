package cmd

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/Atmanity/toolkit/cli/internal/api"
	"github.com/Atmanity/toolkit/cli/internal/sheet"
	"github.com/Atmanity/toolkit/cli/internal/web"
)

// captureTimeout is how long add --wait and pick wait for captures; tests
// shorten it.
var captureTimeout = 15 * time.Minute

// serveTimeoutSeconds is how long select and pick wait for the avatar to
// serve the pick (its config build).
const serveTimeoutSeconds = 300

func (a *app) candidatesCmd() *cobra.Command {
	return group("candidates", "v2: compare the talking clips built for an avatar and pick the one it uses",
		a.candListCmd(), a.candAddCmd(), a.candSelectCmd(), a.candPickCmd(), a.candSheetCmd())
}

func (a *app) candListCmd() *cobra.Command {
	var jsonOut, all bool
	c := &cobra.Command{
		Use:   "list AVATAR_ID",
		Short: "List an avatar's candidates with links to their preview clips.",
		Long: "List the candidates of the avatar's current portrait, voice and preview message (its current version), " +
			"plus the serving one. With --all, also every older version's: selecting a completed one of those goes " +
			"back to that version.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			list := cl.Candidates
			if all {
				list = cl.AllCandidates
			}
			p, err := list(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(p)
			}
			a.printCandidates(p)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	c.Flags().BoolVar(&all, "all", false, "Also list older versions' candidates")
	return c
}

func (a *app) candAddCmd() *cobra.Command {
	var count int
	var wait, jsonOut bool
	c := &cobra.Command{
		Use:     "add AVATAR_ID",
		Short:   "Build more candidates from the same portrait and voice.",
		Example: `  atmee avatars candidates add AVATAR_ID --wait`,
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			// The response lists older captures still queued too: a read just
			// before it tells this request's own captures apart.
			var before *api.CandidatesPage
			if wait {
				if before, err = cl.Candidates(ctx(), args[0]); err != nil {
					return err
				}
			}
			p, err := cl.AddCandidates(ctx(), args[0], count)
			if err != nil {
				return err
			}
			if wait {
				ids := api.NewIDs(before.CandidateList, p.CandidateList)
				if p, err = cl.WaitForCandidates(ctx(), args[0], captureTimeout, pollInterval, ids...); err != nil {
					return err
				}
			}
			if jsonOut {
				return a.printJSON(p)
			}
			a.printCandidates(p)
			return nil
		},
	}
	c.Flags().IntVar(&count, "count", 1, "How many to build, 1-10")
	c.Flags().BoolVar(&wait, "wait", false, "Wait until they are built")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) candSelectCmd() *cobra.Command {
	var jsonOut, wait bool
	var timeout int
	c := &cobra.Command{
		Use:   "select AVATAR_ID CANDIDATE_ID",
		Short: "Choose a candidate by id (without the browser); the avatar uses it a few seconds later.",
		Long: "Choose the candidate the avatar uses, by id (from `atmee avatars candidates list`). It must be finished. " +
			"A finished candidate of an older version (`list --all`) goes back to that version: its portrait, voice " +
			"sample and preview message become the avatar's again, with nothing captured or charged; the persona, " +
			"greeting, guardrails, knowledge and name stay as they are. " +
			"The avatar starts using it a few seconds later; by default the command waits for that.",
		Example: `  atmee avatars candidates select AVATAR_ID CANDIDATE_ID`,
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cid, err := api.ParseCandidateID(args[1])
			if err != nil {
				return usageError{err}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			av, err := cl.SelectCandidate(ctx(), args[0], cid.String())
			if err != nil {
				return err
			}
			if wait {
				w, werr := a.waitServing(cl, args[0], cid.String(), timeout)
				if werr != nil {
					// With --json the state is printed even when the wait
					// failed, so a script sees it.
					if jsonOut && w != nil {
						_ = a.printJSON(w)
					}
					return werr
				}
				av = w
			}
			if jsonOut {
				return a.printJSON(av)
			}
			a.printAvatar(av)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	c.Flags().BoolVar(&wait, "wait", true, "Wait until the avatar uses it (--wait=false to return at once)")
	c.Flags().IntVar(&timeout, "timeout", serveTimeoutSeconds, "Give up waiting after this many seconds")
	return c
}

// waitServing waits until the avatar serves candidateID: the config is built
// after the pick.
func (a *app) waitServing(cl *api.Client, avatarID, candidateID string, timeoutSeconds int) (*api.Avatar, error) {
	a.note("Building the config for the pick...\n")
	return cl.WaitServing(ctx(), avatarID, candidateID, time.Duration(timeoutSeconds)*time.Second, pollInterval)
}

// pickTimeout is how long the picker page waits for a choice.
var pickTimeout = 10 * time.Minute

func (a *app) candPickCmd() *cobra.Command {
	var more, timeout int
	c := &cobra.Command{
		Use:   "pick AVATAR_ID",
		Short: "Compare the candidates side by side in your browser and choose one.",
		Long: "Opens a local page with every candidate's clip side by side. Play them with sound, choose the one whose " +
			"lips match the speech best, or ask for more. The avatar then uses your choice.",
		Example: `  atmee avatars candidates pick AVATAR_ID`,
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			var requested []string // the captures "try more" asked for
			for {
				p, err := cl.WaitForCandidates(ctx(), args[0], captureTimeout, pollInterval, requested...)
				if err != nil {
					return err
				}
				var items []map[string]string
				for _, x := range api.Previewable(p.CandidateList) {
					id := x.CandidateId.String()
					items = append(items, map[string]string{"id": id, "label": id[:8], "url": deref(x.PreviewUrl)})
				}
				if len(items) == 0 {
					return errors.New("no completed candidates")
				}
				choice, err := web.Pick("candidate_picker.html", map[string]any{"items": items}, pickTimeout)
				if err != nil {
					return err
				}
				if choice == nil {
					return errors.New("no selection made")
				}
				if m, _ := choice["more"].(bool); m {
					// Read again, as add does: the list behind the page can
					// be minutes old, and captures added meanwhile by anyone
					// else are not this request's to wait on.
					before, err := cl.Candidates(ctx(), args[0])
					if err != nil {
						return err
					}
					added, err := cl.AddCandidates(ctx(), args[0], more)
					if err != nil {
						return err
					}
					requested = api.NewIDs(before.CandidateList, added.CandidateList)
					a.note("Capturing %d more...\n", more)
					continue
				}
				sel, _ := choice["selected"].(string)
				cid, err := api.ParseCandidateID(sel)
				if err != nil {
					return err
				}
				if _, err := cl.SelectCandidate(ctx(), args[0], cid.String()); err != nil {
					return err
				}
				av, err := a.waitServing(cl, args[0], cid.String(), timeout)
				if err != nil {
					return err
				}
				a.printAvatar(av)
				return nil
			}
		},
	}
	c.Flags().IntVar(&more, "more", 3, "How many to build when you ask for more on the page")
	c.Flags().IntVar(&timeout, "timeout", serveTimeoutSeconds, "Give up waiting for the avatar to use your choice after this many seconds")
	return c
}

func (a *app) candSheetCmd() *cobra.Command {
	var out string
	c := &cobra.Command{
		Use:   "sheet AVATAR_ID",
		Short: "Save one image showing each candidate's mouth over time, to spot silent ones (needs ffmpeg).",
		Long: "Writes a PNG with one row per finished candidate: crops of the mouth over the clip. A row whose lips never " +
			"part is a candidate that will not lip-sync; skip it. Needs ffmpeg.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			p, err := cl.Candidates(ctx(), args[0])
			if err != nil {
				return err
			}
			done := api.Previewable(p.CandidateList)
			if len(done) == 0 {
				return errors.New("no completed candidates yet")
			}
			rows := make([]sheet.Row, len(done))
			for i, x := range done {
				rows[i] = sheet.Row{Label: x.CandidateId.String()[:8], URL: deref(x.PreviewUrl)}
			}
			png, err := sheet.Build(ctx(), rows)
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, png, 0o644); err != nil {
				return err
			}
			a.printf("Wrote %s (%d candidates, top to bottom in list order)\n", out, len(done))
			return nil
		},
	}
	c.Flags().StringVar(&out, "out", "candidates.png", "Image file to write")
	return c
}

// --- sponsor ---

func (a *app) sponsorCmd() *cobra.Command {
	c := group("sponsor", "Pay for visitors' conversations from an avatar's own credit pool (signed-in user, or API key with billing:write)",
		a.sponsorShowCmd(), a.sponsorUpdateCmd(), a.sponsorGrantCmd(), a.sponsorWithdrawCmd())
	// These print JSON either way; --json is accepted like on the other commands.
	c.PersistentFlags().Bool("json", true, "These commands always print JSON")
	_ = c.PersistentFlags().MarkHidden("json")
	return c
}

func (a *app) sponsorShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show AVATAR_ID",
		Short: "Show the sponsor pool's balance and settings.",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			s, err := cl.Sponsor(ctx(), args[0])
			if err != nil {
				return err
			}
			return a.printJSON(s)
		},
	}
}

func (a *app) sponsorUpdateCmd() *cobra.Command {
	var maxPer, daily int
	var exhausted string
	var anon, noAnon, clearMax, clearDaily bool
	c := &cobra.Command{
		Use:   "update AVATAR_ID",
		Short: "Change who the pool pays for and how much (signed-in user, or API key with billing:write).",
		Long: "Change who the pool pays for and how much; one request applies all of its settings or none. --anonymous lets visitors " +
			"without an account spend the pool, so the avatar must be public or unlisted (409 avatar_not_shared) and funded first " +
			"(409 avatar_not_budget_managed: `atmee avatars sponsor grant`). A monetized avatar cannot be sponsored " +
			"(409 monetized_avatar_cannot_be_sponsored).",
		Example: `  atmee avatars update AVATAR_ID --visibility unlisted
  atmee avatars sponsor grant AVATAR_ID 1000
  atmee avatars sponsor update AVATAR_ID --anonymous --anonymous-daily-seconds 300`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			s := map[string]any{}
			if f.Changed("max-per-session") {
				s["maxPerSessionMicrocredits"] = maxPer
			}
			if f.Changed("when-exhausted") {
				s["exhaustedBehavior"] = exhausted
			}
			if anon {
				s["anonymousAccess"] = true
			} else if noAnon {
				s["anonymousAccess"] = false
			}
			if f.Changed("anonymous-daily-seconds") {
				s["anonymousDailySecondsPerVisitor"] = daily
			}
			if clearMax {
				s["maxPerSessionMicrocredits"] = nil
			}
			if clearDaily {
				s["anonymousDailySecondsPerVisitor"] = nil
			}
			if len(s) == 0 {
				return usagef("nothing to update: pass at least one setting flag")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.UpdateSponsor(ctx(), args[0], s)
			if err != nil {
				return err
			}
			return a.printJSON(out)
		},
	}
	f := c.Flags()
	f.IntVar(&maxPer, "max-per-session", 0, "Most the pool pays for one conversation, in microcredits (100 = 1 credit)")
	f.StringVar(&exhausted, "when-exhausted", "", "When the pool is empty: pause (refuse new conversations) or spend_balance (charge your account)")
	f.BoolVar(&anon, "anonymous", false, "Allow anonymous visitors")
	f.BoolVar(&noAnon, "no-anonymous", false, "Disallow anonymous visitors")
	f.IntVar(&daily, "anonymous-daily-seconds", 0, "Daily seconds per anonymous visitor")
	f.BoolVar(&clearMax, "clear-max-per-session", false, "Remove the per-session cap")
	f.BoolVar(&clearDaily, "clear-anonymous-daily-seconds", false, "Remove the daily anonymous limit")
	return c
}

func (a *app) sponsorGrantCmd() *cobra.Command {
	var key string
	c := &cobra.Command{
		Use:     "grant AVATAR_ID MICROCREDITS",
		Short:   "Move credits from your account into the pool (100 microcredits = 1 credit).",
		Example: `  atmee avatars sponsor grant AVATAR_ID 1000   # 10 credits`,
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := strconv.Atoi(args[1])
			if err != nil || n <= 0 {
				return usagef("MICROCREDITS must be a positive whole number")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.GrantSponsor(ctx(), args[0], n, key)
			if err != nil {
				return err
			}
			return a.printJSON(out)
		},
	}
	c.Flags().StringVar(&key, "idempotency-key", "", "Reuse the key a failed grant printed to retry it safely: the same key never pays twice")
	return c
}

func (a *app) sponsorWithdrawCmd() *cobra.Command {
	var n int
	c := &cobra.Command{
		Use:   "withdraw AVATAR_ID",
		Short: "Move credits from the pool back to your account (all that is not in use, by default).",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var amount *int
			if cmd.Flags().Changed("microcredits") {
				amount = &n
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.WithdrawSponsor(ctx(), args[0], amount)
			if err != nil {
				return err
			}
			return a.printJSON(out)
		},
	}
	c.Flags().IntVar(&n, "microcredits", 0, "How much to move back (default: everything not in use)")
	return c
}

// --- sessions ---

func (a *app) sessionCmd() *cobra.Command {
	c := group("session", "Talk to an avatar (chat), or render it into your own LiveKit room (render)",
		a.sessionRenderCmd(), a.sessionStatusCmd(), a.sessionEndCmd(), a.sessionChatCmd())
	// These print JSON either way; --json is accepted like on the other commands.
	c.PersistentFlags().Bool("json", true, "These commands always print JSON")
	_ = c.PersistentFlags().MarkHidden("json")
	return c
}

func (a *app) sessionRenderCmd() *cobra.Command {
	var in api.AvatarSessionInput
	var maxSeconds int
	c := &cobra.Command{
		Use:   "render AVATAR_ID",
		Short: "Bring your own agent: render the avatar into your LiveKit room.",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("max-seconds") {
				in.MaxDurationSeconds = &maxSeconds
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.StartAvatarSession(ctx(), args[0], in)
			if err != nil {
				return err
			}
			return a.printJSON(out)
		},
	}
	f := c.Flags()
	f.StringVar(&in.LiveKitURL, "livekit-url", "", "Your LiveKit server URL (wss://...)")
	f.StringVar(&in.LiveKitToken, "livekit-token", "", "A token you mint for the avatar participant in your room")
	f.StringVar(&in.AgentIdentity, "agent-identity", "", "Your agent's participant identity (checked against the token)")
	f.IntVar(&maxSeconds, "max-seconds", 0, "Longest the session may run, in seconds")
	f.BoolVar(&in.WaitJoined, "wait-joined", false, "Return once the avatar is in your room (up to 10 min on a cold start)")
	_ = c.MarkFlagRequired("livekit-url")
	_ = c.MarkFlagRequired("livekit-token")
	return c
}

func (a *app) sessionStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status SESSION_ID",
		Short: "Show a render session's status (from `session render`).",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.AvatarSessionStatus(ctx(), args[0])
			if err != nil {
				return err
			}
			return a.printJSON(out)
		},
	}
}

func (a *app) sessionEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "end SESSION_ID",
		Short: "End a render session and stop its billing (safe to repeat).",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.EndAvatarSession(ctx(), args[0])
			if err != nil {
				return err
			}
			return a.printJSON(out)
		},
	}
}

func (a *app) sessionChatCmd() *cobra.Command {
	var in api.EmbedSessionInput
	var maxSeconds int
	var checkInAfter float64
	var open bool
	var tools appToolFlags
	c := &cobra.Command{
		Use:   "chat AVATAR_ID",
		Short: "Start a conversation with an avatar; --open joins it in your browser.",
		Long: "Start a conversation: the avatar joins a LiveKit room and the answer carries the room and a user token. " +
			"Join within 60 seconds or the session ends. --open joins for you: it opens the room in LiveKit Meet in " +
			"your browser (ATMEE_NO_BROWSER only prints the link).\n\n" +
			"--tools declares functions the avatar may call in this conversation (a JSON array of up to 30; see the handbook), " +
			"with --tool-instructions, one text for all of them, saying how and when to use them. Every call reaches the visitor's LiveKit client as the RPC " +
			"`atmee.tool`: the page that joins the room answers it.",
		Example: `  atmee avatars session chat AVATAR_ID --open
  atmee avatars session chat AVATAR_ID --max-seconds 300
  atmee avatars session chat AVATAR_ID --tools tools.json --tool-instructions how-to-use-them.txt`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("max-seconds") {
				in.MaxDurationSeconds = &maxSeconds
			}
			if err := tools.apply(&in, a.in); err != nil {
				return err
			}
			if in.Tools != nil {
				// LiveKit Meet answers no atmee.tool RPC.
				a.note("Warning: tool calls go to the participant that joins with the user token, and LiveKit Meet (the link " +
					"below, and --open) does not answer them: await calls fail and notify calls are lost. Join with your own " +
					"LiveKit client that registers the atmee.tool RPC (see the handbook).\n")
			}
			if cmd.Flags().Changed("check-in-after") {
				in.CheckInAfterS = &checkInAfter
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.StartSession(ctx(), args[0], in)
			if err != nil {
				return err
			}
			link := meetURL(out.ServerUrl, out.UserToken)
			if !open {
				// This session's own link: running the command again would start
				// another session, not join this one.
				a.note("Join within 60 seconds or the session ends. Open this link to join in your browser:\n  %s\n"+
					"(--open opens it for you.)\n", link)
			} else {
				a.note("Opening the room in your browser; join within 60 seconds or the session ends:\n  %s\n", link)
				if err := browse(link); err != nil {
					a.note("Could not open the browser (%v); open the link above.\n", err)
				}
			}
			// The room above is for testing an app. Someone who only wants to
			// talk to the avatar needs its chat page on the website, which the
			// start returns beside the room (absent from a server that
			// predates it).
			if out.Links != nil {
				a.note("To just talk to it, without a session of your own, share its chat page on the website:\n  %s\n", out.Links.Chat)
			}
			return a.printJSON(out)
		},
	}
	f := c.Flags()
	f.BoolVar(&open, "open", false, "Open the room in LiveKit Meet in your browser")
	f.IntVar(&maxSeconds, "max-seconds", 0, "Longest the conversation may run, in seconds")
	f.StringVar(&in.VisitorID, "visitor-id", "", "Remember this visitor across conversations: an unguessable id your server derives")
	f.StringVar(&in.Language, "language", "", "Language for this conversation: en, zh-TW, ja, ... (default: the avatar's)")
	f.BoolVar(&in.NoWelcome, "no-welcome", false, "Start without the avatar's self-introduction: it waits for the user to speak (v2 avatars)")
	f.Float64Var(&checkInAfter, "check-in-after", 0, "Seconds of silence before the avatar checks in, 0 to 3600, 0 for no check-in (v2 avatars; default: about 10)")
	tools.register(c)
	return c
}

// meetURL is LiveKit Meet's custom-server page with the room's server and the
// user token filled in: opening it joins the room with camera and microphone.
func meetURL(serverURL, token string) string {
	return "https://meet.livekit.io/custom?" + url.Values{"liveKitUrl": {serverURL}, "token": {token}}.Encode()
}
