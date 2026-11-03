package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

func (a *app) speechCmd() *cobra.Command {
	return group("speech", "Speech-to-speech sessions over a WebSocket, without video (start, get, end)",
		a.speechStartCmd(), a.speechGetCmd(), a.speechEndCmd())
}

func (a *app) speechStartCmd() *cobra.Command {
	var in api.SpeechSessionInput
	var avatar, settingsPath, instructionsPath string
	var maxSeconds int
	var jsonOut bool
	c := &cobra.Command{
		Use:   "start --avatar AVATAR_ID",
		Short: "Reserve a speech session with an avatar; a client connects to its url with its token within 60 seconds.",
		Long: "Reserve a speech session (speech-to-speech, audio only) with an avatar: it uses the avatar's persona and always its voice " +
			"(an avatar without a voice answers avatar_has_no_voice). The answer carries the WebSocket url and a token " +
			"that admits one connection within 60 seconds, presented as `Authorization: Bearer <token>` or as the WebSocket " +
			"subprotocols `atmee-live` and `atmee-token.<token>`. The token is printed only with --json, for handing to the " +
			"client that connects.\n\n" +
			"--settings FILE pins session.start fields from your backend (a JSON object: instructions, persona, facts, scenario, agenda, " +
			"directive, avatar_name, user_name, self_introduction, input, delegation, avatar_timezone, user_timezone): the client " +
			"cannot set them, and pinned persona fields override the avatar's. --instructions FILE pins the instructions alone.\n\n" +
			"The session ends at its endsAt (the reservation plus the length granted, which the key, plan or balance may cap below " +
			"--max-seconds), whenever the connection opened; the first connection holds it for good, so a client that lost its " +
			"connection starts a new session. Billing runs by the minute from the speech server's first usage report (within 30 seconds " +
			"of the connection). A session nobody connects to costs nothing and is closed (failed, no_user_joined) by the account's " +
			"next read or start once its token has been expired 30 seconds, otherwise by the stale-session cleanup within about 5 minutes. " +
			"The CLI streams no audio.",
		Example: `  atmee speech start --avatar AVATAR_ID --max-seconds 300
  atmee speech start --avatar AVATAR_ID --instructions instructions.txt --json
  atmee speech start --avatar AVATAR_ID --settings pinned.json --language fr-FR --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if avatar == "" {
				return usagef("--avatar is required: the avatar whose persona and voice the session uses")
			}
			id, err := api.ParseID(avatar)
			if err != nil {
				return usagef("--avatar: %v", err)
			}
			in.AvatarID = id
			if cmd.Flags().Changed("max-seconds") {
				if maxSeconds < 60 || maxSeconds > 3600 {
					return usagef("--max-seconds takes 60 to 3600")
				}
				in.MaxDurationSeconds = &maxSeconds
			}
			if in.Settings, err = a.speechSettings(settingsPath, instructionsPath); err != nil {
				return err
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			s, err := cl.StartSpeechSession(ctx(), in)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(s)
			}
			a.printf("Speech session %s\n", s.SessionId)
			a.printf("  URL           %s\n", s.Url)
			a.printf("  Token         expires %s (in %s); --json prints it for the client that connects\n",
				stamp(s.TokenExpiresAt), time.Until(s.TokenExpiresAt).Round(time.Second))
			length := fmt.Sprintf("%s from now", seconds(s.ActualDurationSeconds))
			if s.RequestedDurationSeconds != nil {
				if s.DurationCapped {
					length += fmt.Sprintf(" (capped: %s asked; the key, plan or balance allows less)", seconds(*s.RequestedDurationSeconds))
				} else {
					length += " (as asked)"
				}
			}
			a.printf("  Length        %s\n", length)
			a.printf("  Billing       %s\n", s.BillingMode)
			a.printf("End it with `atmee speech end %s`.\n", s.SessionId)
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&avatar, "avatar", "", "The avatar whose persona and voice the session uses (required)")
	f.IntVar(&maxSeconds, "max-seconds", 0, "Longest the session may run, from the reservation, 60 to 3600 (default 600)")
	f.StringVar(&in.Language, "language", "", "BCP 47 language of the conversation, e.g. en or fr-FR")
	f.StringVar(&settingsPath, "settings", "", "A JSON file (- for stdin) of session.start fields to pin, e.g. {\"instructions\": \"...\", \"user_name\": \"Sam\"}")
	f.StringVar(&instructionsPath, "instructions", "", "A text file (- for stdin) of instructions to pin")
	f.BoolVar(&jsonOut, "json", false, "Print the API's JSON, the connection token included")
	return c
}

// speechSettings builds the pinned settings from --settings and
// --instructions; nil when neither is given.
func (a *app) speechSettings(settingsPath, instructionsPath string) (json.RawMessage, error) {
	if settingsPath == "" && instructionsPath == "" {
		return nil, nil
	}
	if settingsPath == "-" && instructionsPath == "-" {
		return nil, usagef("--settings and --instructions cannot both read stdin")
	}
	fields := map[string]json.RawMessage{}
	if settingsPath != "" {
		raw, err := a.readInput(settingsPath)
		if err != nil {
			return nil, err
		}
		if fields, err = api.ParseSpeechSettings(raw); err != nil {
			return nil, usagef("--settings %s: %v", settingsPath, err)
		}
	}
	if instructionsPath != "" {
		if v, set := fields["instructions"]; set && string(bytes.TrimSpace(v)) != "null" {
			return nil, usagef("--instructions and the settings file's \"instructions\" both pin the instructions; give one")
		}
		raw, err := a.readInput(instructionsPath)
		if err != nil {
			return nil, err
		}
		text := strings.TrimSpace(string(raw))
		if text == "" {
			return nil, usagef("--instructions %s is empty", instructionsPath)
		}
		b, _ := json.Marshal(text)
		fields["instructions"] = b
	}
	return json.Marshal(fields)
}

// seconds is a length in seconds as a duration (10m0s).
func seconds(n int) time.Duration { return time.Duration(n) * time.Second }

func (a *app) speechGetCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "get SESSION_ID",
		Short: "Show a speech session's state, end reason and cost.",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			s, err := cl.SpeechSessionStatus(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(s)
			}
			a.printf("Speech session %s  %s\n", s.SessionId, s.Status)
			if s.AvatarId != nil {
				a.printf("  Avatar        %s\n", s.AvatarId)
			}
			if s.Language != nil {
				a.printf("  Language      %s\n", *s.Language)
			}
			if s.ConnectedAt != nil {
				a.printf("  Billed from   %s\n", stamp(*s.ConnectedAt))
			} else {
				a.printf("  Billed from   not yet (no usage report)\n")
			}
			a.printf("  Ends          %s at the latest (%s granted)\n", stamp(s.EndsAt), seconds(s.ActualDurationSeconds))
			if s.EndRequestedAt != nil && s.EndedAt == nil {
				a.printf("  End requested %s; it ends at the connection's next usage report (within 30 seconds), billed until then\n",
					stamp(*s.EndRequestedAt))
			}
			if s.EndedAt != nil {
				reason := ""
				if s.EndReason != nil {
					reason = " (" + *s.EndReason + ")"
				}
				a.printf("  Ended         %s%s\n", stamp(*s.EndedAt), reason)
			}
			if s.BilledSeconds != nil || s.ChargedMicrocredits != nil {
				a.printf("  Cost          %s credits for %s (%s)\n", credits(int64(deref(s.ChargedMicrocredits))),
					time.Duration(deref(s.BilledSeconds))*time.Second, s.BillingMode)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) speechEndCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "end SESSION_ID",
		Short: "End a speech session (safe to repeat).",
		Long: "End a speech session (completed, ended_by_api). With no connection open it settles at once. With a connection open it " +
			"ends at the connection's next usage report, within 30 seconds, and is billed until then: the answer is still active, " +
			"with the time the end was requested, and `atmee speech get` reads it completed once it settled. " +
			"A repeat keeps the first request; ending a settled session reports that it already ended.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			e, err := cl.EndSpeechSession(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(e)
			}
			switch {
			case e.AlreadyEnded:
				a.printf("Speech session %s had already ended.\n", args[0])
			case e.Status == avatarapi.SpeechSessionEndResponseStatusActive:
				requested := ""
				if e.EndRequestedAt != nil {
					requested = " (requested " + stamp(*e.EndRequestedAt) + ")"
				}
				a.printf("Ending speech session %s%s: a connection is open, so it ends at its next usage report, within 30 seconds, "+
					"and is billed until then. `atmee speech get %s` shows it completed once it settled.\n", args[0], requested, args[0])
			default:
				a.printf("Ended speech session %s (%s).\n", args[0], e.Status)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}
