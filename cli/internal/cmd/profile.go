package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

// publicNeeds maps what public still needs (the API's `missing`) to the
// command that gives it.
var publicNeeds = map[string]string{
	"title": "--title", "tags": "--tag", "subtitle": "--subtitle", "description": "--description",
}

// explain adds what to do next to the API's refusals of a share,
// statement or monetization change; other errors pass through.
func explain(err error, id string) error {
	var e *api.Error
	if !errors.As(err, &e) {
		return err
	}
	var hint string
	switch e.Code {
	case "public_requirements_missing":
		var flags []string
		thumb := false
		for _, m := range e.Missing {
			if m == "thumbnail" {
				thumb = true
			} else if fl, ok := publicNeeds[m]; ok {
				flags = append(flags, fl)
			}
		}
		var steps []string
		if len(flags) > 0 {
			steps = append(steps, "atmee avatars update "+id+" "+strings.Join(flags, " ... ")+" ...")
		}
		if thumb {
			steps = append(steps, "atmee avatars thumbnail set "+id+" IMAGE")
		}
		if len(steps) == 0 {
			return err
		}
		hint = "give them with: " + strings.Join(steps, " and ")
	case "moderation_blocked":
		hint = "the avatar has not passed moderation"
		if e.ModerationState != "" {
			hint += " (" + e.ModerationState + ")"
		}
		hint += "; it can be unlisted or public once it has. Try again later, or keep it private"
	case "avatar_is_earning":
		hint = "the statement cannot change while the avatar is monetized: add --monetize off to the same command"
	case "statement_required":
		hint = "monetizing needs a statement of who the avatar depicts: --statement fictional, or self or representative (verified in Creator Studio)"
	case "identity_not_verified":
		hint = "a self or representative statement needs Creator Studio's identity check before monetizing; for a fictional character use --statement fictional"
	case "payouts_not_ready":
		hint = "monetizing needs a verified payout account, set up in Creator Studio"
		if e.PayoutsStatus != "" {
			hint += " (payouts: " + e.PayoutsStatus + ")"
		}
		hint += "; `atmee account` shows its status"
	case "monetization_eligibility_not_met":
		hint = "the last 30 days do not meet the requirements yet: atmee avatars monetization " + id
	case "monetization_limit_reached":
		hint = "the plan's cap on monetized avatars is reached"
		if e.Cap != nil && e.MonetizingAvatars != nil {
			hint += fmt.Sprintf(" (%d of %d)", *e.MonetizingAvatars, *e.Cap)
		}
		hint += "; turn monetization off on another avatar, or change plan"
	case "sponsor_pool_not_empty":
		hint = fmt.Sprintf("the sponsor pool holds %s credits, which turning monetization on withdraws to the account: re-run with --confirm-sponsor-withdrawal",
			credits(e.SponsorPoolMicrocredits))
	default:
		return err
	}
	return fmt.Errorf("%w\n  %s", err, hint)
}

func (a *app) monetizationCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "monetization AVATAR_ID",
		Short: "Show whether an avatar is monetized, its 30-day eligibility, the plan's cap and its earnings.",
		Long: "What Creator Studio's monetization panel shows: whether monetization is on, the last 30 days (your own " +
			"conversations left out) against the eligibility requirements, the account's cap on monetized avatars, and the " +
			"earnings in credits (100 microcredits = 1 credit; the current cycle is the account's billing cycle). " +
			"Turn it on or off with `atmee avatars update AVATAR_ID --monetize on|off`.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			m, err := cl.Monetization(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(m)
			}
			a.printMonetization(m)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func tick(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func (a *app) printMonetization(m *api.Monetization) {
	a.printf("Monetized    %s\n", yesNo(m.Monetized))
	a.printf("Eligible     %s\n", yesNo(m.Eligible))
	l, r := m.Last30Days, m.Requirements
	a.printf("Last 30 days %s %d of %d conversations · %s %.2f of %d minutes · %s %d of %d people\n",
		tick(r.MeetsConversations), l.Conversations, r.Conversations, tick(r.MeetsMinutes), l.Minutes, r.Minutes,
		tick(r.MeetsUniqueUsers), l.UniqueUsers, r.UniqueUsers)
	cp := m.AccountCap
	limit := "no plan cap"
	if cp.Cap != nil {
		limit = fmt.Sprintf("plan cap %d", *cp.Cap)
	}
	atCap := ""
	if cp.AtCap {
		atCap = " (at the cap: this avatar cannot be monetized)"
	}
	a.printf("Account      %d monetized · %s%s\n", cp.MonetizingAvatars, limit, atCap)
	e := m.Earnings
	a.printf("Earnings     %s credits this cycle (%s) · %s credits all time (%s)\n",
		credits(e.CurrentCycleMicrocredits), plural(int(e.CurrentCycleSessions), "session"),
		credits(e.AllTimeMicrocredits), plural(int(e.AllTimeSessions), "session"))
}

func (a *app) questionnaireCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "questionnaire AVATAR_ID",
		Short: "List Creator Studio's knowledge questions, numbered, with the avatar's answers.",
		Long: "List the questions of Creator Studio's knowledge questionnaire in Studio's order, numbered, with the " +
			"avatar's answers. Answer one with `atmee avatars update AVATAR_ID --answer N=ANSWER` (N the number, or the " +
			"question's text); the other answers stay. --clear questionnaire removes them all.",
		Args: exactArgs(1),
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
				return a.printJSON(map[string]any{"questions": k.Questions, "questionnaire": k.Questionnaire})
			}
			if len(k.Questions) == 0 {
				a.printf("No questions.\n")
				return nil
			}
			for i, q := range k.Questions {
				a.printf("%d. %s\n", i+1, q)
				if ans, ok := k.Questionnaire[q]; ok {
					a.printf("   %s\n", strings.ReplaceAll(ans, "\n", "\n   "))
				} else {
					a.printf("   (not answered)\n")
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "Print the questions and the answers as JSON (questions, questionnaire)")
	return c
}

func (a *app) thumbnailCmd() *cobra.Command {
	var jsonOut bool
	set := &cobra.Command{
		Use:   "set AVATAR_ID IMAGE",
		Short: "Set the Explore thumbnail from a local image (jpg, png or webp, at most 10 MB).",
		Long: "Set the avatar's Explore thumbnail from a local image (jpg, png or webp, at most 10 MB), as Creator " +
			"Studio's share card does. It replaces any thumbnail, and no automatic one is made for the avatar after it. " +
			"Nothing is rebuilt.",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			av, err := cl.SetThumbnail(ctx(), args[0], args[1])
			if err != nil {
				return explain(err, args[0])
			}
			if jsonOut {
				return a.printJSON(av)
			}
			a.printf("Thumbnail set.\n")
			return nil
		},
	}
	set.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	var confirm, rmJSON bool
	remove := &cobra.Command{
		Use:   "remove AVATAR_ID",
		Short: "Remove the Explore thumbnail (asks first; --yes skips the question).",
		Long: "Remove the avatar's Explore thumbnail, custom or automatic: Explore shows the portrait instead, and an " +
			"automatic thumbnail may be made again. A public avatar needs a thumbnail: make it unlisted or private first.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := api.ParseID(args[0]); err != nil {
				return err
			}
			if !confirm {
				if err := a.confirm(fmt.Sprintf("Remove the thumbnail of avatar %s? [y/N]", args[0])); err != nil {
					return err
				}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			av, err := cl.RemoveThumbnail(ctx(), args[0])
			if e := (*api.Error)(nil); errors.As(err, &e) && e.Code == "public_requirements_missing" {
				return fmt.Errorf("%w\n  a public avatar needs its thumbnail: make it unlisted or private first (atmee avatars update %s --visibility unlisted)", err, args[0])
			}
			if err != nil {
				return explain(err, args[0])
			}
			if rmJSON {
				return a.printJSON(av)
			}
			a.printf("Thumbnail removed.\n")
			return nil
		},
	}
	remove.Flags().BoolVar(&confirm, "yes", false, "Remove without asking")
	remove.Flags().BoolVar(&rmJSON, "json", false, jsonHelp)
	return group("thumbnail", "Set or remove the avatar's Explore thumbnail.", set, remove)
}
