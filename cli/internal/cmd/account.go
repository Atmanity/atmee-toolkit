package cmd

import (
	"context"
	"encoding/json"
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
			"the plan, top-up and coupon buckets, what running sessions hold, and the credits that expire next. One kind of credit pays for every model (V2 HD, V2, V1), each at its rate. " +
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
			"and whether payouts are set up (monetizing an avatar needs them verified). It shows the account by id (and by name " +
			"where the API returns one) and how it was picked. " +
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
				return a.printAccountJSON(acct)
			}
			a.printAccount(acct)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// printAccountJSON is the API's JSON, with the fields accountFacts adds.
func (a *app) printAccountJSON(x *api.Account) error {
	if a.accountFacts == nil {
		return a.printJSON(x)
	}
	_, facts := a.accountFacts(x)
	if facts == nil {
		return a.printJSON(x)
	}
	b, err := json.Marshal(x)
	if err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for k, v := range facts {
		m[k] = v
	}
	return a.printJSON(m)
}

// helpCenter is Creator Studio's help center, where an account contacts
// us: on the website the account's links point to, or named without an
// address when the server reports no links.
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

// v2TrialLine is the account's V2 trial window, where the server reports
// one (limits.v2Trial), or "" when it reports none, no trial is offered
// (days 0) or a state this CLI does not know.
func v2TrialLine(t *api.V2Trial) string {
	if t == nil || (t.Days != nil && *t.Days <= 0) {
		return ""
	}
	on := func(p *time.Time) string {
		if p == nil {
			return "?"
		}
		return day(*p)
	}
	switch t.State {
	case "not_started":
		return "starts " + on(t.StartsAt) + ", ends " + on(t.EndsAt)
	case "active":
		return "on until " + on(t.EndsAt) + " (started " + on(t.StartsAt) + ")"
	case "ended":
		return "ended " + on(t.EndsAt)
	}
	return ""
}

// v2UnlockLine is the account's unlock outside its plan (limits.v2Unlock)
// as a labelled line: a timed one reads as a trial ("V2 trial", "Atmee V2
// and V2 HD trial ends {date}", or "Atmee V2 HD trial ends {date}" where V2
// stays after it: permanentLevel), a permanent one as "V2 unlock", "Atmee V2
// and V2 HD unlocked". Both are "" when the server reports nothing unlocked
// or a level this CLI does not know.
func v2UnlockLine(u *avatarapi.AccountV2Unlock) (label, text string) {
	if u == nil {
		return "", ""
	}
	var what string
	switch u.Level {
	case avatarapi.AccountV2UnlockLevelV2:
		what = "Atmee V2"
	case avatarapi.AccountV2UnlockLevelV2hd:
		what = "Atmee V2 and V2 HD"
	default:
		return "", ""
	}
	keep := ""
	if u.PermanentLevel != nil {
		keep = string(*u.PermanentLevel)
	}
	// What stays covers the unlock's level: nothing ends.
	if keep == string(avatarapi.AccountV2UnlockPermanentLevelV2hd) || keep == string(u.Level) {
		return "V2 unlock", what + " unlocked"
	}
	if u.ExpiresAt != nil {
		// V2 stays once a timed V2 HD ends: only V2 HD is a trial.
		if keep == string(avatarapi.AccountV2UnlockPermanentLevelV2) {
			what = "Atmee V2 HD"
		}
		return "V2 trial", what + " trial ends " + day(*u.ExpiresAt)
	}
	return "V2 unlock", what + " unlocked"
}

func (a *app) printAccount(x *api.Account) {
	a.printf("Account %s\n", x.AccountId)
	a.printAccountSource(x)
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
	if p := a.planText(x); p != "" {
		plan = p
	}
	a.printf("Plan         %s\n", plan)
	if pp := p.PendingPlan; pp != nil {
		a.printf("Next cycle   %s (%s)\n", deref(pp.Name), deref(pp.Code))
	}

	l := x.Limits
	access := "on"
	if !l.ApiAccess {
		access = "OFF: switched off for this account by an admin (sessions answer 403 api_access_disabled); contact support"
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
			api.V2AccessInviteRequired: "can't build: Atmee V2 needs Pro and up; creates default to V1 without it. Existing V2 avatars keep working, and anyone can talk to any avatar",
			api.V2AccessUnavailable:    "not offered on this server",
		}[string(*v)])
	}
	trial := v2TrialLine(x.V2Trial)
	if trial != "" {
		a.printf("V2 trial     %s\n", trial)
	}
	if label, u := v2UnlockLine(x.Limits.V2Unlock); u != "" {
		// Beside the plan's trial window, the unlock keeps its own label
		// so the two dates are not both "V2 trial".
		if trial != "" {
			label = "V2 unlock"
		}
		a.printf("%-13s%s\n", label, u)
	}
	a.printf("Limits       session length %s · %s at once · %s API avatars\n", length, limit(l.MaxConcurrentSessions), limit(l.MaxApiAvatars))

	pr := x.Pricing
	// A server older than the v2 conversation rate leaves it out: its one
	// conversation rate then covers both engines.
	conversation := creditsPerMinute(pr.SessionPerMinuteMicrocredits)
	if x.SessionV2Reported {
		conversation = "v1 " + conversation + ", v2 " + creditsPerMinute(pr.SessionV2PerMinuteMicrocredits)
	}
	// Atmee V2 HD's own rates, where the server reports them; otherwise its
	// price is left unsaid rather than taken for v2's.
	if v := pr.SessionV2HdPerMinuteMicrocredits; v != nil {
		conversation += ", V2 HD " + creditsPerMinute(*v)
	}
	render := "v1 " + creditsPerMinute(pr.RenderV1PerMinuteMicrocredits) + ", v2 " + creditsPerMinute(pr.RenderV2PerMinuteMicrocredits)
	if v := pr.RenderV2HdPerMinuteMicrocredits; v != nil {
		render += ", V2 HD " + creditsPerMinute(*v)
	}
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
	// Atmee V2 HD, where this server reports it: new V2 HD avatars are for
	// Growth and up (an admin can switch an account on or off), which
	// limits.v2Pro says. Its sizes show either way, for existing V2 HD
	// avatars too.
	if v := l.V2Pro; v != nil {
		switch {
		case !*v:
			a.printf("V2 HD        can't build: Atmee V2 HD needs Growth and up; --engine v2hd answers 403 v2hd_access_required without it. Existing V2 HD avatars keep working\n")
		case x.V2Access != nil && string(*x.V2Access) != api.V2AccessGranted:
			a.printf("V2 HD        yes, where Atmee V2 is on this account (--engine v2hd)\n")
		default:
			a.printf("V2 HD        yes (--engine v2hd)\n")
		}
		if rs := l.V2ProResolutions; rs != nil && len(*rs) > 0 {
			sizes := make([]string, len(*rs))
			for i, r := range *rs {
				sizes[i] = fmt.Sprintf("%dx%d", r.Width, r.Height)
			}
			a.printf("V2 HD sizes  %s (--resolution)\n", strings.Join(sizes, ", "))
		}
	}
	if v := l.VideoGeneration; v != nil && !*v {
		a.printf("Video clips  turned off for this account: contact us through %s (atmee generate clip answers 403 video_generation_not_in_plan)\n", helpCenter(x.Links))
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
		Long: "Show who the commands act as and on which Avatar API: the signed-in user, the account (by id, and by name where " +
			"the API returns one) and how it was picked, or the API key's name, scopes, avatar restriction and spend cap. They come from the API (GET /v1/api_keys/current); when it cannot be reached, " +
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
					out["account_source"] = a.accountSource()
					if facts := a.accountJSON(); facts != nil {
						out["account"] = facts
					}
				} else {
					out["key_error"] = a.rename(lookupErr.Error())
					if choices := ambiguousAccounts(lookupErr); len(choices) > 0 {
						out["accounts"] = choices
					}
				}
				if err := a.printJSON(out); err != nil {
					return err
				}
				if accountProblem(lookupErr) {
					return lookupErr
				}
				return nil
			}
			if key != "" {
				a.printf("API key %s -> %s\n", maskKey(key), base)
			} else {
				a.printf("Signed in as %s -> %s\n", orDash(sess.Email), sess.BaseURL)
			}
			if accountProblem(lookupErr) {
				return lookupErr
			}
			if cur == nil {
				a.note("Could not read who this is from the API (%s); showing the saved credentials only.\n", a.rename(describeLookup(lookupErr)))
				a.noteAccounts(lookupErr)
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
		a.printAccountUsed()
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
	a.printAccountUsed()
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
		Short: "Redeem a code: credits, or access to Atmee V2 or V2 HD.",
		Long: "Redeem a code: credits, or access to Atmee V2 or V2 HD. Redeems it for your account, as Creator " +
			"Studio's Billing page does. Each account redeems a code once. Needs `atmee login`: an API key cannot " +
			"redeem codes.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
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
			// What the account may create now (not only what this code added):
			// v2Pro (a current server) says V2 HD, v2Access says V2.
			v2 := res.V2Access == nil || *res.V2Access == api.V2AccessGranted
			switch {
			case res.V2Pro != nil && *res.V2Pro && v2:
				parts = append(parts, "this account can build Atmee V2 and V2 HD")
			case res.V2Access != nil:
				parts = append(parts, map[string]string{
					api.V2AccessGranted:        "this account can build Atmee V2",
					api.V2AccessInviteRequired: "it did not unlock building Atmee V2 (Atmee V2 needs Pro and up)",
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
