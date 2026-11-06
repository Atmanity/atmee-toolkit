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
	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
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
			"the plan, top-up and coupon buckets, how much is usable on Atmee V2 (where the server reports it), what running sessions hold, and the credits that expire next. " +
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
	// The part that pays for Atmee V2 (v2 avatars, their sessions, video
	// clips, and existing V2 HD avatars), where the server reports it.
	if v := b.V2SpendableMicrocredits; v != nil {
		a.printf("Usable on Atmee V2  %s\n", credits(*v))
	}
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

// helpCenter is Creator Studio's help center, where Enterprise customers
// contact us: on the website the account's links point to, or named
// without an address when the server reports no links.
func helpCenter(links *avatarapi.AccountLinks) string {
	const name = "Creator Studio's help center"
	if links == nil {
		return name
	}
	u, err := url.Parse(links.Billing)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return name
	}
	return name + " (" + u.Scheme + "://" + u.Host + "/studio/help-center)"
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
			api.V2AccessGranted:        "yes",
			api.V2AccessInviteRequired: "not unlocked by an invite code: Atmee V2 comes with Growth and up; below that, `atmee redeem CODE` with an invite code unlocks it (creates default to V1 without it)",
			api.V2AccessUnavailable:    "not offered on this server",
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
	// Atmee V2 HD's rates are left unsaid even where the server reports
	// them: it is for Enterprise customers on custom plans (--json keeps them).
	render := "v1 " + creditsPerMinute(pr.RenderV1PerMinuteMicrocredits) + ", v2 " + creditsPerMinute(pr.RenderV2PerMinuteMicrocredits)
	a.printf("Per minute   conversation %s · avatar session %s\n", conversation, render)
	if pr.SpeechPerMinuteMicrocredits != nil {
		a.printf("             speech session %s\n", creditsPerMinute(*pr.SpeechPerMinuteMicrocredits))
	}
	a.printf("Preview clip %s credits\n", credits(int64(pr.PreviewClipMicrocredits)))
	if g := pr.Generation; g != nil {
		clip := ""
		// A server older than per-size clip prices reports one price per 15
		// seconds (or per second); a newer one lists each size's price below.
		if g.ClipSizes == nil {
			if g.ClipPer15SecondsMicrocredits != nil {
				clip = " · clip " + credits(int64(*g.ClipPer15SecondsMicrocredits)) + " per 15 s"
			} else if g.ClipPerSecondMicrocredits != nil {
				clip = " · clip " + credits(int64(*g.ClipPerSecondMicrocredits)) + "/s"
			}
		}
		a.printf("Generation   portraits %s · enhance %s · voice %s%s credits\n", credits(int64(g.PortraitsMicrocredits)),
			credits(int64(g.EnhancementsMicrocredits)), credits(int64(g.VoicesMicrocredits)), clip)
		// The plan's free generation: portraits, enhance and voice draw on
		// it before credits; clips never do.
		if fa := g.FreeAllowance; fa != nil && fa.LimitMicrocredits > 0 {
			resets := ""
			if fa.ResetsAt != nil {
				resets = ", resets " + day(*fa.ResetsAt)
			}
			a.printf("Free gen     %s of %s credits left this cycle (%s used%s) · portraits, enhance and voice; not clips\n",
				credits(int64(fa.RemainingMicrocredits)), credits(int64(fa.LimitMicrocredits)), credits(int64(fa.UsedMicrocredits)), resets)
		}
		if cp := g.ClipPrices; cp != nil && len(*cp) > 0 {
			a.printClipPrices(*cp)
		} else if cs := g.ClipSizes; cs != nil && len(*cs) > 0 {
			// A server that priced each size the same for any length.
			sizes := make([]string, len(*cs))
			for i, c := range *cs {
				sizes[i] = fmt.Sprintf("%dx%d %s", c.Width, c.Height, credits(int64(c.Microcredits)))
			}
			a.printf("Clip sizes   %s credits a clip, any length from 1 to 15 s (--resolution)\n", strings.Join(sizes, ", "))
		}
	}
	// The v2 avatar sizes: limits.v2Resolutions, or on an older server
	// (no clipSizes) its clipResolutions, which then listed them.
	var v2Sizes []string
	if rs := l.V2Resolutions; rs != nil {
		for _, r := range *rs {
			v2Sizes = append(v2Sizes, fmt.Sprintf("%dx%d", r.Width, r.Height))
		}
	} else if g := pr.Generation; g != nil && g.ClipSizes == nil && g.ClipResolutions != nil {
		for _, r := range *g.ClipResolutions {
			v2Sizes = append(v2Sizes, fmt.Sprintf("%dx%d", r.Width, r.Height))
		}
	}
	if len(v2Sizes) > 0 {
		a.printf("v2 sizes     %s (--resolution)\n", strings.Join(v2Sizes, ", "))
	}
	// Atmee V2 HD, where this server reports it: for Enterprise customers,
	// whatever limits.v2Pro says. Its sizes stay for existing V2 HD avatars.
	if l.V2Pro != nil {
		a.printf("V2 HD        Enterprise only, on custom plans: contact us through %s\n", helpCenter(x.Links))
		if rs := l.V2ProResolutions; rs != nil && len(*rs) > 0 {
			sizes := make([]string, len(*rs))
			for i, r := range *rs {
				sizes[i] = fmt.Sprintf("%dx%d", r.Width, r.Height)
			}
			a.printf("V2 HD sizes  %s (--resolution)\n", strings.Join(sizes, ", "))
		}
	}
	if v := l.VideoGeneration; v != nil && !*v {
		a.printf("Video clips  not in this plan (Growth and up): atmee generate clip answers 403 video_generation_not_in_plan\n")
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
			"unlock Atmee V2, or both (Growth and up include Atmee V2 already). Each account redeems a code once. " +
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
					api.V2AccessGranted:        "Atmee V2 is unlocked",
					api.V2AccessInviteRequired: "it did not unlock Atmee V2 (Growth and up include it; below that, an invite code unlocks it)",
					api.V2AccessUnavailable:    "v2 avatars are not offered on this server",
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

// printClipPrices lists the clip sizes with their prices by length tier:
// "480x480 0.10/0.30/0.50" for up to 5/10/15 s.
func (a *app) printClipPrices(cp []avatarapi.ClipSizePrices) {
	var upTo []string
	for _, p := range cp[0].Prices {
		upTo = append(upTo, strconv.Itoa(p.UpToSeconds))
	}
	sizes := make([]string, len(cp))
	for i, c := range cp {
		prices := make([]string, len(c.Prices))
		for j, p := range c.Prices {
			prices[j] = credits(int64(p.Microcredits))
		}
		sizes[i] = fmt.Sprintf("%dx%d %s", c.Width, c.Height, strings.Join(prices, "/"))
	}
	a.printf("Clip sizes   %s credits a clip up to %s s, charged at the next length up (--resolution)\n",
		strings.Join(sizes, ", "), strings.Join(upTo, "/"))
}
