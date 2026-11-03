package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

// clearable maps the --clear names to their API fields ("knowledge.x" is
// nested under knowledge). The API removes a value when the field is sent
// empty ("", [] or {}); an absent field is left unchanged. An empty preview
// message is the default sentence, an empty statement resets it. name,
// visibility, language, the who/what prompts and self-intro cannot be
// emptied (the API refuses an empty value).
var clearable = map[string]string{
	"description": "description", "greeting": "greeting", "alias": "preferredAlias",
	"preview-message": "previewMessage", "guardrails": "guardrails", "facts": "knowledge.facts",
	"spoken-name": "spokenName", "known-aliases": "knownAliases", "habits": "conversationalHabits",
	"title": "title", "subtitle": "subtitle", "tags": "tags", "socials": "socialLinks",
	"statement": "statementType", "questionnaire": "knowledge.questionnaire",
}

// updateFlagHelp describes the update command's string flags.
var updateFlagHelp = map[string]string{
	"name":            "Avatar name",
	"description":     "A short description of the avatar (at most 280 characters; shown on Explore)",
	"who-prompt":      whoPromptHelp,
	"what-prompt":     whatPromptHelp,
	"greeting":        "The first thing it says in a conversation",
	"self-intro":      "How it introduces itself (more than 50 characters)",
	"alias":           "The short name it goes by in conversation",
	"language":        "Language it speaks: en, zh-TW, ja, ...",
	"visibility":      "private, unlisted or public (public needs a title, a tag, a subtitle, a description and a thumbnail)",
	"spoken-name":     spokenNameHelp,
	"preview-message": "v2: the line new candidates speak (at most 300 characters; builds new candidates)",
	"title":           "The title on Explore (at most 100 characters; without one Explore shows the alias, then the name)",
	"subtitle":        "The line under the title on Explore (at most 200 characters)",
}

// updateStringFlags are the update command's string flags and their API fields.
var updateStringFlags = map[string]string{
	"name": "name", "description": "description", "visibility": "visibility", "language": "language",
	"who-prompt": "whoPrompt", "what-prompt": "whatPrompt", "greeting": "greeting", "self-intro": "selfIntroduction",
	"alias": "preferredAlias", "preview-message": "previewMessage", "spoken-name": "spokenName",
	"title": "title", "subtitle": "subtitle",
}

// Studio's share card limits, refused here before any request.
const (
	maxTags       = 5
	maxSocialURL  = 500
	statementHelp = "Who the avatar depicts: self (you), representative (someone you represent) or fictional; " +
		"self and representative are verified in Creator Studio"
)

var socialTypes = []string{"instagram", "tiktok", "facebook", "twitter", "youtube"}

var statementTypes = map[string]string{
	"self": "self_verification", "representative": "representative_verification", "fictional": "fictional_verification",
}

// checkTags trims the tags and drops case-insensitive duplicates, as the
// API does, and refuses what it would refuse.
func checkTags(tags []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if !strings.ContainsFunc(t, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			return nil, usagef("--tag %q: a tag needs a letter or a digit", t)
		}
		if k := strings.ToLower(t); !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	if len(out) > maxTags {
		return nil, usagef("at most %d tags (got %d)", maxTags, len(out))
	}
	return out, nil
}

// parseSocials reads TYPE=URL settings into the API's socialLinks.
func parseSocials(settings []string) ([]map[string]string, error) {
	out := []map[string]string{}
	seen := map[string]bool{}
	for _, s := range settings {
		typ, url, ok := strings.Cut(strings.TrimSpace(s), "=")
		typ, url = strings.ToLower(strings.TrimSpace(typ)), strings.TrimSpace(url)
		if !ok || !slices.Contains(socialTypes, typ) {
			return nil, usagef("--social %q: use TYPE=URL, TYPE one of %s", s, strings.Join(socialTypes, ", "))
		}
		if !strings.HasPrefix(url, "https://") || len(url) <= len("https://") || len(url) > maxSocialURL {
			return nil, usagef("--social %q: the link must be an https URL of at most %d characters", s, maxSocialURL)
		}
		if seen[typ] {
			return nil, usagef("--social: %s given twice (one link per platform)", typ)
		}
		seen[typ] = true
		out = append(out, map[string]string{"type": typ, "url": url})
	}
	return out, nil
}

// parseAnswers applies QUESTION=ANSWER settings over the current answers.
// QUESTION is one of Studio's questions, or its number in that list; an
// empty answer removes it.
func parseAnswers(settings []string, questions []string, current map[string]string, avatarID string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range current {
		out[k] = v
	}
	for _, s := range settings {
		q, ans, ok := strings.Cut(s, "=")
		q, ans = strings.TrimSpace(q), strings.TrimSpace(ans)
		if !ok || q == "" {
			return nil, usagef("--answer %q: use QUESTION=ANSWER (QUESTION as `atmee avatars questionnaire %s` lists it, or its number)", s, avatarID)
		}
		if n, err := strconv.Atoi(q); err == nil {
			if n < 1 || n > len(questions) {
				return nil, usagef("--answer %q: there are %d questions (`atmee avatars questionnaire %s`)", s, len(questions), avatarID)
			}
			q = questions[n-1]
		} else if !slices.Contains(questions, q) {
			return nil, usagef("--answer %q: not one of Studio's questions (`atmee avatars questionnaire %s` lists them)", s, avatarID)
		}
		if ans == "" {
			delete(out, q)
			continue
		}
		if n := len([]rune(ans)); n > 2000 {
			return nil, usagef("--answer %q: at most 2,000 characters (got %d)", q, n)
		}
		out[q] = ans
	}
	return out, nil
}

func (a *app) updateCmd() *cobra.Command {
	var jsonOut, confirmWithdrawal bool
	var factsFile, statement, monetize string
	var guardrails, knownAliases, habits, tags, socials, answers, clear []string
	c := &cobra.Command{
		Use:   "update AVATAR_ID",
		Short: "Change an avatar's name, persona, knowledge, Explore profile, visibility or monetization.",
		Long: "Change fields of an avatar; only the flags you pass change. v1 rebuilds its persona and is briefly not ready. " +
			"v2 keeps serving while the persona rebuilds; a new --preview-message or --resolution builds new candidates.\n\n" +
			"The Explore profile (--title, --subtitle, --description, --tag, --social, the thumbnail of " +
			"`atmee avatars thumbnail set`) is what Creator Studio's \"Share to community\" card edits; nothing is rebuilt. " +
			"--visibility public needs a title, a tag, a subtitle, a description and a thumbnail.\n\n" +
			"--monetize on needs a statement (--statement fictional, or a self or representative statement verified in " +
			"Creator Studio), payouts set up in Creator Studio (`atmee account`), the 30-day requirements " +
			"(`atmee avatars monetization`) and room under the plan's cap. When the sponsor pool holds credits it asks " +
			"first, since monetizing withdraws them to the account (--confirm-sponsor-withdrawal answers yes).\n\n" +
			"--answer merges into the questionnaire answers: the others stay.",
		Example: `  atmee avatars update AVATAR_ID --greeting "Hello again!"
  atmee avatars update AVATAR_ID --who-prompt "A patient chess coach" --what-prompt "Teach an opening"
  atmee avatars update AVATAR_ID --facts-file ./facts.txt
  atmee avatars update AVATAR_ID --title "Chess with Ada" --subtitle "Openings, explained" --tag chess --tag coach \
    --social youtube=https://youtube.com/@ada --visibility public
  atmee avatars update AVATAR_ID --statement fictional --monetize on
  atmee avatars update AVATAR_ID --answer 1="Openings and endgames"
  atmee avatars update AVATAR_ID --clear greeting --clear tags`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			id := args[0]
			fields := map[string]any{} // API field ("knowledge.x" nested) -> value
			setBy := map[string]string{}
			set := func(flag, key string, v any) {
				fields[key] = v
				setBy[key] = flag
			}
			for flag, key := range updateStringFlags {
				if f.Changed(flag) {
					v, _ := f.GetString(flag)
					if strings.TrimSpace(v) == "" {
						if _, ok := clearable[flag]; ok {
							return usagef("--%s is empty: use --clear %s to remove the value", flag, flag)
						}
						return usagef("--%s cannot be empty", flag)
					}
					set(flag, key, v)
				}
			}
			if f.Changed("guardrail") {
				set("guardrail", "guardrails", guardrails)
			}
			if f.Changed("known-alias") {
				if err := checkKnownAliases(knownAliases); err != nil {
					return err
				}
				set("known-alias", "knownAliases", knownAliases)
			}
			if f.Changed("tag") {
				t, err := checkTags(tags)
				if err != nil {
					return err
				}
				set("tag", "tags", t)
			}
			if f.Changed("social") {
				sl, err := parseSocials(socials)
				if err != nil {
					return err
				}
				set("social", "socialLinks", sl)
			}
			if f.Changed("statement") {
				v, ok := statementTypes[strings.ToLower(strings.TrimSpace(statement))]
				if !ok {
					return usagef("--statement takes self, representative or fictional (--clear statement resets it)")
				}
				set("statement", "statementType", v)
			}
			if f.Changed("monetize") {
				var on bool
				switch strings.ToLower(strings.TrimSpace(monetize)) {
				case "on":
					on = true
				case "off":
				default:
					return usagef("--monetize takes on or off")
				}
				m := map[string]any{"enabled": on}
				if confirmWithdrawal {
					if !on {
						return usagef("--confirm-sponsor-withdrawal goes with --monetize on")
					}
					m["confirmSponsorWithdrawal"] = true
				}
				set("monetize", "monetization", m)
			} else if confirmWithdrawal {
				return usagef("--confirm-sponsor-withdrawal goes with --monetize on")
			}
			if f.Changed("runtime-seed") {
				seed, _ := f.GetInt("runtime-seed")
				if err := api.CheckRuntimeSeed(seed); err != nil {
					return usagef("%v", err)
				}
				set("runtime-seed", "runtimeSeed", seed)
			}
			if f.Changed("resolution") {
				v, _ := f.GetString("resolution")
				r, err := api.ParseResolution(v)
				if err != nil {
					return usagef("%v", err)
				}
				set("resolution", "resolution", map[string]int{"width": r.Width, "height": r.Height})
			}
			if f.Changed("facts") || factsFile != "" {
				facts, _ := f.GetString("facts")
				flag := "facts"
				if factsFile != "" {
					b, err := os.ReadFile(factsFile)
					if err != nil {
						return usagef("could not read the input file: %v", err)
					}
					facts = string(b)
					if !f.Changed("facts") {
						flag = "facts-file"
					}
				}
				if strings.TrimSpace(facts) == "" {
					return usagef("--facts is empty: use --clear facts to remove the value")
				}
				set(flag, "knowledge.facts", facts)
			}
			// Merged after the read below, but they contradict --clear now.
			if f.Changed("habit") {
				setBy["conversationalHabits"] = "habit"
			}
			if f.Changed("answer") {
				setBy["knowledge.questionnaire"] = "answer"
			}
			cleared := map[string]bool{}
			for _, name := range clear {
				key, ok := clearable[name]
				if !ok {
					return usagef("--clear takes one of: %s", strings.Join(sortedKeys(clearable), ", "))
				}
				if cleared[key] {
					continue // --clear x given twice
				}
				if flag, dup := setBy[key]; dup {
					return usagef("--%s and --clear %s contradict each other", flag, name)
				}
				cleared[key] = true
				switch key {
				case "guardrails", "knownAliases", "tags", "socialLinks":
					fields[key] = []string{}
				case "conversationalHabits", "knowledge.questionnaire":
					fields[key] = map[string]string{}
				default:
					fields[key] = ""
				}
			}
			if len(fields) == 0 && !f.Changed("habit") && !f.Changed("answer") {
				return usagef("nothing to update: pass at least one field flag")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			if f.Changed("habit") {
				// The API replaces the whole set, so the current sliders are
				// read first and only the named ones change. Bad levels fail
				// before that read.
				if _, err := parseHabits(habits, nil); err != nil {
					return err
				}
				cur, err := cl.Get(ctx(), id)
				if err != nil {
					return err
				}
				var current map[string]string
				if cur.Persona != nil && cur.Persona.ConversationalHabits != nil {
					current = map[string]string{}
					for k, v := range *cur.Persona.ConversationalHabits {
						current[k] = string(v)
					}
				}
				h, err := parseHabits(habits, current)
				if err != nil {
					return err
				}
				fields["conversationalHabits"] = h
			}
			if f.Changed("answer") {
				// The same for the questionnaire: the current answers and
				// Studio's questions come from get-knowledge.
				k, err := cl.Knowledge(ctx(), id)
				if err != nil {
					return err
				}
				ans, err := parseAnswers(answers, k.Questions, k.Questionnaire, id)
				if err != nil {
					return err
				}
				fields["knowledge.questionnaire"] = ans
			}
			body := map[string]any{}
			knowledge := map[string]any{}
			for k, v := range fields {
				if sub, ok := strings.CutPrefix(k, "knowledge."); ok {
					knowledge[sub] = v
				} else {
					body[k] = v
				}
			}
			if len(knowledge) > 0 {
				body["knowledge"] = knowledge
			}
			if r, ok := fields["resolution"].(map[string]int); ok {
				// The size is checked for the avatar's engine; an avatar
				// that cannot be read is left to the server.
				if av, err := cl.Get(ctx(), id); err == nil {
					if err := cl.CheckSize(ctx(), engine(deref(av.Engine)), api.Resolution{Width: r["width"], Height: r["height"]}); err != nil {
						var pro *api.V2ProSizeError
						if errors.As(err, &pro) {
							return usagef("%v, and an avatar's engine can't change: create a V2 Pro avatar for it "+
								"(`atmee avatars create --engine v2pro`, where the server offers it), or pick a v2 size (`atmee account` lists them)", err)
						}
						return usagef("%v", err)
					}
				}
			}
			av, err := cl.Update(ctx(), id, body)
			var apiErr *api.Error
			if errors.As(err, &apiErr) && apiErr.Code == "sponsor_pool_not_empty" && !confirmWithdrawal {
				// Monetizing withdraws the pool to the account: ask, as
				// delete does, and send again with the confirmation.
				q := fmt.Sprintf("The sponsor pool holds %s credits; turning monetization on withdraws them to the account. Withdraw them and turn monetization on? [y/N]",
					credits(apiErr.SponsorPoolMicrocredits))
				if a.confirm(q) != nil {
					return fmt.Errorf("not monetized: the sponsor pool holds %s credits. Re-run with --confirm-sponsor-withdrawal to withdraw them to the account and turn monetization on",
						credits(apiErr.SponsorPoolMicrocredits))
				}
				body["monetization"] = map[string]any{"enabled": true, "confirmSponsorWithdrawal": true}
				av, err = cl.Update(ctx(), id, body)
			}
			if err != nil {
				return explain(err, id)
			}
			if jsonOut {
				return a.printJSON(av)
			}
			a.printAvatar(av)
			return nil
		},
	}
	f := c.Flags()
	for flag := range updateStringFlags {
		f.String(flag, "", updateFlagHelp[flag])
	}
	f.StringArrayVar(&guardrails, "guardrail", nil, "A rule it must follow (repeat for several; replaces the whole list)")
	f.StringArrayVar(&knownAliases, "known-alias", nil, knownAliasHelp+" (repeat for several; replaces the whole list)")
	f.StringArrayVar(&habits, "habit", nil, habitHelp+"; the others keep their level")
	f.String("facts", "", factsHelp+"; replaces the current text")
	f.StringVar(&factsFile, "facts-file", "", "Read --facts from a text file")
	f.StringArrayVar(&answers, "answer", nil, "Answer a question of Studio's knowledge questionnaire as QUESTION=ANSWER, QUESTION as atmee avatars questionnaire lists it, or its number "+
		"(repeat for several; the other answers stay; an empty ANSWER removes that answer)")
	f.StringArrayVar(&tags, "tag", nil, "A tag on Explore (repeat for several, at most 5; replaces the whole set)")
	f.StringArrayVar(&socials, "social", nil, "A social link as TYPE=URL, TYPE one of instagram, tiktok, facebook, twitter, youtube and an https URL (repeat for several; replaces the whole set)")
	f.StringVar(&statement, "statement", "", statementHelp)
	f.StringVar(&monetize, "monetize", "", "Turn monetization on or off (on needs a statement, verified payouts, the 30-day requirements and room under the plan's cap)")
	f.BoolVar(&confirmWithdrawal, "confirm-sponsor-withdrawal", false, "With --monetize on: withdraw the sponsor pool's credits to the account without asking")
	f.Int("runtime-seed", 0, runtimeSeedHelp)
	f.String("resolution", "", resolutionHelp)
	f.StringArrayVar(&clear, "clear", nil, "Remove a value: description, greeting, alias, spoken-name, known-aliases, habits, guardrails, facts, questionnaire, "+
		"title, subtitle, tags, socials, statement (resets it) or preview-message (back to the default line); repeat for several")
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	promptAliases(c)
	return c
}
