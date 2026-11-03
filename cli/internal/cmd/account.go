package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

// credits formats microcredits as credits (100 microcredits = 1 credit),
// with thousands separators: 4998500 -> "49,985.00".
func credits(micro int64) string {
	sign := ""
	if micro < 0 {
		sign, micro = "-", -micro
	}
	whole := strconv.FormatInt(micro/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s.%02d", sign, b.String(), micro%100)
}

// creditsPerMinute is a price in microcredits per minute, in credits.
func creditsPerMinute(micro int) string { return credits(int64(micro)) + "/min" }

func day(t time.Time) string { return t.Local().Format("2006-01-02") }

// stamp is a time in the local zone, RFC 3339.
func stamp(t time.Time) string { return t.Local().Format(time.RFC3339) }

func bucketName(b string) string {
	switch b {
	case "PLAN":
		return "plan"
	case "TOPUP":
		return "top-up"
	case "COUPON":
		return "coupon"
	}
	return strings.ToLower(b)
}

func (a *app) balanceCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "balance",
		Short: "Credits left on the account, and what expires when.",
		Long: "Credits left on your account (the signed-in user's or the API key's; 100 microcredits = 1 credit): what a session can spend now, " +
			"the plan, top-up and coupon buckets, what running sessions hold, and the credits that expire next. " +
			"Works for a signed-in user, or a key with the sessions or avatars:read scope.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			b, err := cl.Balance(ctx())
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(b)
			}
			a.printBalance(b)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) printBalance(b *api.Balance) {
	if b.Unlimited {
		a.printf("Unlimited: sessions on this account are not charged.\n")
	}
	a.printf("Spendable  %s credits\n", credits(b.SpendableMicrocredits))
	coupon := ""
	if !b.CouponSpendable {
		coupon = "  (not spent by this key: metered keys use plan and top-up credits only)"
	}
	a.printf("  Plan     %s\n  Top-up   %s\n  Coupon   %s%s\n", credits(b.PlanMicrocredits), credits(b.TopupMicrocredits), credits(b.CouponMicrocredits), coupon)
	if b.HeldByLiveSessionsMicrocredits > 0 {
		a.printf("Held by running sessions  %s (what they do not use comes back when they end)\n", credits(b.HeldByLiveSessionsMicrocredits))
	}
	if len(b.NextExpiry) == 0 {
		a.printf("Nothing expires.\n")
		return
	}
	next := b.NextExpiry[0]
	a.printf("Next expiry  %s %s credits on %s\n", credits(next.AmountMicrocredits), bucketName(next.Bucket), day(next.ExpiresAt))
	for _, e := range b.NextExpiry[1:] {
		a.printf("             %s %s credits on %s\n", credits(e.AmountMicrocredits), bucketName(e.Bucket), day(e.ExpiresAt))
	}
}

func (a *app) accountCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "account",
		Short: "The account's plan, limits, prices per minute, capture quota and payout status.",
		Long: "The plan your account is on (the signed-in user's or the API key's; and any change scheduled for the next cycle), the limits on API usage, " +
			"what a session minute and a preview clip cost, how many v2 captures are queued or running against the cap, " +
			"and whether payouts are set up (monetizing an avatar needs them verified). " +
			"Works for a signed-in user, or a key with the sessions or avatars:read scope.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			acct, err := cl.Account(ctx())
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(acct)
			}
			a.printAccount(acct)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) printAccount(x *api.Account) {
	a.printf("Account %s\n", x.AccountId)
	p := x.Plan
	plan := deref(p.Name)
	if plan == "" {
		plan = "no plan"
	}
	if code := deref(p.Code); code != "" {
		plan += " (" + code + ")"
	}
	if p.IncludedMicrocredits != nil {
		plan += " · " + credits(*p.IncludedMicrocredits) + " credits a cycle"
	}
	if p.CycleEnd != nil {
		plan += " · cycle ends " + day(*p.CycleEnd)
	}
	a.printf("Plan         %s\n", plan)
	if pp := p.PendingPlan; pp != nil {
		a.printf("Next cycle   %s (%s)\n", deref(pp.Name), deref(pp.Code))
	}

	l := x.Limits
	access := "on"
	if !l.ApiAccess {
		access = "OFF (sessions answer 403 api_access_disabled)"
	}
	length := "no plan cap"
	if l.MaxSessionSeconds != nil {
		length = (time.Duration(*l.MaxSessionSeconds) * time.Second).String()
	}
	limit := func(p *int) string {
		if p == nil {
			return "no plan cap"
		}
		return strconv.Itoa(*p)
	}
	a.printf("API access   %s\n", access)
	if v := x.V2Access; v != nil {
		a.printf("v2 avatars   %s\n", map[string]string{
			"granted":         "yes",
			"invite_required": "invite only: redeem an invite code in Creator Studio (Billing); creates default to v1",
			"unavailable":     "not offered on this server",
		}[string(*v)])
	}
	a.printf("Limits       session length %s · %s at once · %s API avatars\n", length, limit(l.MaxConcurrentSessions), limit(l.MaxApiAvatars))

	pr := x.Pricing
	// A server older than the v2 conversation rate leaves it out: its one
	// conversation rate then covers both engines.
	conversation := creditsPerMinute(pr.SessionPerMinuteMicrocredits)
	if x.SessionV2Reported {
		conversation = "v1 " + conversation + ", v2 " + creditsPerMinute(pr.SessionV2PerMinuteMicrocredits)
	}
	a.printf("Per minute   conversation %s · avatar session v1 %s, v2 %s\n",
		conversation, creditsPerMinute(pr.RenderV1PerMinuteMicrocredits), creditsPerMinute(pr.RenderV2PerMinuteMicrocredits))
	if pr.SpeechPerMinuteMicrocredits != nil {
		a.printf("             speech session %s\n", creditsPerMinute(*pr.SpeechPerMinuteMicrocredits))
	}
	a.printf("Preview clip %s credits\n", credits(int64(pr.PreviewClipMicrocredits)))
	if g := pr.Generation; g != nil {
		clip := ""
		if g.ClipPer15SecondsMicrocredits != nil {
			clip = " · clip " + credits(int64(*g.ClipPer15SecondsMicrocredits)) + " per 15 s"
		} else if g.ClipPerSecondMicrocredits != nil {
			clip = " · clip " + credits(int64(*g.ClipPerSecondMicrocredits)) + "/s"
		}
		a.printf("Generation   portraits %s · enhance %s · voice %s%s credits\n", credits(int64(g.PortraitsMicrocredits)),
			credits(int64(g.EnhancementsMicrocredits)), credits(int64(g.VoicesMicrocredits)), clip)
	}
	if g := pr.Generation; g != nil && g.ClipResolutions != nil && len(*g.ClipResolutions) > 0 {
		sizes := make([]string, len(*g.ClipResolutions))
		for i, r := range *g.ClipResolutions {
			sizes[i] = fmt.Sprintf("%dx%d", r.Width, r.Height)
		}
		a.printf("v2 sizes     %s (--resolution)\n", strings.Join(sizes, ", "))
	}
	if g := pr.Generation; g != nil && g.ClipAnySize != nil {
		if *g.ClipAnySize {
			a.printf("Clip sizes   any, up to 1920x1080 and 30 s (1920x1080: 10 s, 1280x720: 22 s)\n")
		} else {
			a.printf("Clip sizes   the v2 sizes only, up to 15 s\n")
		}
	}
	q := x.CaptureQuota
	a.printf("Captures     %d of %d queued or running\n", q.Pending, q.PendingLimit)
	if po := x.Payouts; po != nil {
		status := strings.ReplaceAll(string(po.Status), "_", " ")
		switch {
		case po.Status == "verified" && po.VerifiedAt != nil:
			status += " since " + day(*po.VerifiedAt)
		case po.Status != "verified":
			status += " (set up payouts in Creator Studio to monetize avatars)"
		}
		a.printf("Payouts      %s\n", status)
	}
}

// whoamiTimeout bounds the key lookup, so an offline whoami answers fast.
var whoamiTimeout = 10 * time.Second

func (a *app) whoamiCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "whoami",
		Short: "Show who you are signed in as, and which API the commands use.",
		Long: "Show who the commands act as and on which Avatar API: the signed-in user and account, or the API key's name, scopes, " +
			"avatar restriction and spend cap. They come from the API (GET /v1/api_keys/current); when it cannot be reached, " +
			"the saved key or sign-in and the base URL are shown alone.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, sess, base := a.store.APIKey(), a.store.Session(), a.baseURL()
			if key == "" && sess == nil {
				if jsonOut {
					return a.printJSON(map[string]any{"api_key": false, "signed_in": false, "base_url": base})
				}
				a.printf("Not signed in. Run `atmee login` or set ATMEE_API_KEY.\n")
				return nil
			}
			cur, lookupErr := a.currentKey()
			if jsonOut {
				out := map[string]any{"api_key": key != "", "signed_in": key == "", "base_url": base}
				if key != "" {
					out["saved"] = maskKey(key)
				} else {
					out["email"] = sess.Email
					out["signed_in_to"] = sess.BaseURL
				}
				if cur != nil {
					out["key"] = cur
				} else {
					out["key_error"] = lookupErr.Error()
				}
				return a.printJSON(out)
			}
			if key != "" {
				a.printf("API key %s -> %s\n", maskKey(key), base)
			} else {
				a.printf("Signed in as %s -> %s\n", orDash(sess.Email), sess.BaseURL)
			}
			if cur == nil {
				a.note("Could not read who this is from the API (%s); showing the saved credentials only.\n", describeLookup(lookupErr))
				return nil
			}
			a.printCurrentKey(cur)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (a *app) currentKey() (*api.CurrentUser, error) {
	cl, err := a.api()
	if err != nil {
		return nil, err
	}
	c, cancel := context.WithTimeout(ctx(), whoamiTimeout)
	defer cancel()
	return cl.CurrentKey(c)
}

// describeLookup says why the key could not be read: offline, or what the
// API answered.
func describeLookup(err error) string {
	var ue *url.Error
	var ne net.Error
	switch {
	case errors.As(err, &ue) || errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded):
		return "offline or unreachable: " + err.Error()
	}
	return err.Error()
}

func (a *app) printCurrentKey(k *api.CurrentUser) {
	if string(k.Kind) == "user" {
		a.printf("Signed-in user %s on account %s\n", deref(k.UserId), k.AccountId)
		return
	}
	name := deref(k.Name)
	if name == "" {
		name = "(unnamed)"
	}
	state := ""
	if k.Active != nil && !*k.Active {
		state = " (inactive)"
	}
	a.printf("Key          %s%s  %s\n", name, state, deref(k.Id))
	a.printf("Account      %s\n", k.AccountId)
	scopes := "-"
	if k.Scopes != nil && len(*k.Scopes) > 0 {
		scopes = strings.Join(*k.Scopes, ", ")
	}
	a.printf("Scopes       %s\n", scopes)
	avatar := "any avatar of the account"
	if k.AvatarId != nil {
		avatar = "only " + *k.AvatarId
	}
	a.printf("Avatars      %s\n", avatar)
	if k.BillingMode != nil {
		a.printf("Billing      %s\n", *k.BillingMode)
	}
	if sc := k.SpendCap; sc != nil {
		window := map[string]string{"daily": "today", "monthly": "this month", "total": "in total"}[sc.Window]
		if window == "" {
			window = sc.Window
		}
		line := credits(sc.SpentMicrocredits) + " credits spent " + window
		if sc.LimitMicrocredits != nil {
			line = fmt.Sprintf("%s of %s credits used %s, %s left", credits(sc.SpentMicrocredits), credits(*sc.LimitMicrocredits), window,
				credits(deref(sc.RemainingMicrocredits)))
		}
		if sc.WindowResetsAt != nil {
			line += " (resets " + day(*sc.WindowResetsAt) + ")"
		}
		if !sc.Enforced {
			line += "; not enforced on this plan"
		}
		a.printf("Spend cap    %s\n", line)
	}
	if k.MaxSessionSeconds != nil || k.MaxConcurrentSessions != nil {
		parts := []string{}
		if k.MaxSessionSeconds != nil {
			parts = append(parts, "sessions up to "+(time.Duration(*k.MaxSessionSeconds)*time.Second).String())
		}
		if k.MaxConcurrentSessions != nil {
			parts = append(parts, strconv.Itoa(*k.MaxConcurrentSessions)+" at once")
		}
		a.printf("Key limits   %s\n", strings.Join(parts, " · "))
	}
}

func (a *app) redeemCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "redeem CODE",
		Short: "Redeem an invite or credit code for the account.",
		Long: "Redeems a code for your account, as Creator Studio's Billing page does. A code can add credits, " +
			"open v2 avatars (invite-only for now), or both; each account redeems a code once. " +
			"Needs `atmee login`: an API key cannot redeem codes.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			res, err := cl.Redeem(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(res)
			}
			parts := []string{}
			if res.MicrocreditsGranted > 0 {
				parts = append(parts, credits(res.MicrocreditsGranted)+" credits added")
			}
			if res.V2Access != nil {
				parts = append(parts, map[string]string{
					"granted":         "v2 avatars are open",
					"invite_required": "v2 avatars are still invite-only for this account",
					"unavailable":     "v2 avatars are not offered on this server",
				}[string(*res.V2Access)])
			}
			if len(parts) == 0 {
				parts = append(parts, "done")
			}
			a.printf("Redeemed %s: %s.\n", strings.TrimSpace(args[0]), strings.Join(parts, "; "))
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "print the API's JSON")
	return c
}
