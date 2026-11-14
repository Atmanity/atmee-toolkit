package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCredits(t *testing.T) {
	for micro, want := range map[int64]string{0: "0.00", 5: "0.05", 100: "1.00", 4998500: "49,985.00", 123456789: "1,234,567.89", -250: "-2.50"} {
		if got := credits(micro); got != want {
			t.Errorf("credits(%d) = %q, want %q", micro, got, want)
		}
	}
}

// accountAPI answers the account reads as the Avatar API does; account, if
// given, replaces the GET /v1/account body.
func accountAPI(t *testing.T, account ...string) *httptest.Server {
	t.Helper()
	routes := map[string]string{
		"/v1/account/balance": `{"wallet":"direct","spendableMicrocredits":4998500,"planMicrocredits":98500,"topupMicrocredits":4900000,
			"couponMicrocredits":700,"couponSpendable":false,"heldByLiveSessionsMicrocredits":300,"unlimited":false,
			"nextExpiry":[{"bucket":"COUPON","amountMicrocredits":700,"expiresAt":"2026-10-30T12:00:00Z"},
				{"bucket":"PLAN","amountMicrocredits":98500,"expiresAt":"2026-11-01T12:00:00Z"}]}`,
		"/v1/account": `{"accountId":"11111111-1111-4111-8111-111111111111",
			"plan":{"code":"scale","name":"Scale","includedMicrocredits":100000,"cycleEnd":"2026-11-01T12:00:00Z","pendingPlan":{"code":"pro_2026","name":"Pro"}},
			"limits":{"apiAccess":true,"maxConcurrentSessions":7,"maxApiAvatars":50},
			"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
				"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0,
				"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100,"clipPerSecondMicrocredits":10,
					"clipResolutions":[{"width":512,"height":512},{"width":384,"height":640}],"clipAnySize":true}},
			"captureQuota":{"pending":2,"pendingLimit":10},"payouts":{"status":"verified","verifiedAt":"2026-09-01T12:00:00Z"}}`,
		"/v1/api_keys/current": `{"kind":"api_key","id":"k-1","accountId":"11111111-1111-4111-8111-111111111111","name":"prod backend",
			"scopes":["sessions","avatars:read"],"active":true,"maxSessionSeconds":600,
			"spendCap":{"limitMicrocredits":50000,"window":"monthly","spentMicrocredits":1250,"remainingMicrocredits":48750,
				"windowResetsAt":"2026-11-01T12:00:00Z","enforced":true}}`,
	}
	if len(account) > 0 {
		routes["/v1/account"] = account[0]
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "sk_atmee_x" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBalance(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t).URL)
	out, errOut, code := run(t, "", "balance")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Spendable  49,985.00 credits", "Plan     985.00", "Top-up   49,000.00", "Coupon   7.00",
		"not spent by this key", "Held by running sessions  3.00", "Next expiry  7.00 coupon credits on 2026-10-3", "985.00 plan credits on 2026-11-0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	out, _, _ = run(t, "", "balance", "--json")
	var b map[string]any
	if err := json.Unmarshal([]byte(out), &b); err != nil || b["spendableMicrocredits"] != 4998500.0 || b["couponSpendable"] != false {
		t.Fatalf("--json: %v %s", err, out)
	}
}

func TestAccount(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Account 11111111-1111-4111-8111-111111111111", "Scale (scale) · 1,000.00 credits a cycle · cycle ends 2026-11-0",
		"Next cycle   Pro (pro_2026)", "API access   on", "session length no plan cap · 7 at once · 50 API avatars",
		"conversation 1.00/min · avatar session v1 0.50/min, v2 1.50/min", "Generation   portraits 1.00 · enhance 1.00 · voice 1.00 · clip 0.10/s credits", "Preview clip 0.00 credits", "v2 sizes     512x512, 384x640", "Captures     2 of 10 queued or running", "Payouts      verified since 2026-09-0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	out, _, _ = run(t, "", "account", "--json")
	if !strings.Contains(out, `"renderV2PerMinuteMicrocredits": 150`) || !strings.Contains(out, `"status": "verified"`) ||
		strings.Contains(out, "V2PerMinuteMicrocredits\": 0") {
		t.Fatalf("--json: %s", out)
	}
}

// A server with per-size clip prices lists each size and its price per
// clip, not the deprecated per-15-seconds price, the v2 avatar sizes from
// limits.v2Resolutions (not the clip sizes), and a plan without video
// generation says it is turned off for the account.
func TestAccountClipSizes(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},
		"limits":{"apiAccess":true,"videoGeneration":false,"v2Resolutions":[{"width":512,"height":512},{"width":384,"height":640}]},
		"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
			"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0,
			"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100,
				"clipPer15SecondsMicrocredits":2250,"clipPerSecondMicrocredits":150,
				"clipSizes":[{"width":480,"height":480,"microcredits":50},{"width":1280,"height":720,"microcredits":150}],
				"clipResolutions":[{"width":480,"height":480},{"width":1280,"height":720}],"clipAnySize":false}},
		"captureQuota":{"pending":0,"pendingLimit":10}}`).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Generation   portraits 1.00 · enhance 1.00 · voice 1.00 credits",
		"Clip sizes   480x480 0.50, 1280x720 1.50 credits a clip, any length from 1 to 15 s",
		"Video clips  turned off for this account: contact us through Creator Studio's help center", "v2 sizes     512x512, 384x640 (--resolution)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "per 15 s") || strings.Contains(out, "1280x720 (--resolution)") {
		t.Errorf("deprecated clip fields shown:\n%s", out)
	}
}

// A server that prices clips by length lists each size's tiers.
func TestAccountClipSizesTiered(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},
		"limits":{"apiAccess":true,"videoGeneration":false,"v2Resolutions":[{"width":512,"height":512},{"width":384,"height":640}]},
		"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
			"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0,
			"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100,
				"clipPer15SecondsMicrocredits":2250,"clipPerSecondMicrocredits":150,
				"clipSizes":[{"width":480,"height":480,"microcredits":50},{"width":1280,"height":720,"microcredits":200}],"clipPrices":[{"width":480,"height":480,"prices":[{"upToSeconds":5,"microcredits":10},{"upToSeconds":10,"microcredits":30},{"upToSeconds":15,"microcredits":50}]},{"width":1280,"height":720,"prices":[{"upToSeconds":5,"microcredits":80},{"upToSeconds":10,"microcredits":140},{"upToSeconds":15,"microcredits":200}]}],
				"clipResolutions":[{"width":480,"height":480},{"width":1280,"height":720}],"clipAnySize":false}},
		"captureQuota":{"pending":0,"pendingLimit":10}}`).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Generation   portraits 1.00 · enhance 1.00 · voice 1.00 credits",
		"Clip sizes   480x480 0.10/0.30/0.50, 1280x720 0.80/1.40/2.00 credits a clip up to 5/10/15 s, charged at the next length up",
		"Video clips  turned off for this account: contact us through Creator Studio's help center", "v2 sizes     512x512, 384x640 (--resolution)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "per 15 s") || strings.Contains(out, "1280x720 (--resolution)") {
		t.Errorf("deprecated clip fields shown:\n%s", out)
	}
}

// The plan's free generation allowance shows what is left, used and when
// it resets; a plan without one shows nothing.
func TestAccountFreeGeneration(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	body := func(fa string) string {
		return `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},"limits":{"apiAccess":true},
			"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
				"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0,
				"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100` + fa + `}},
			"captureQuota":{"pending":0,"pendingLimit":10}}`
	}
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, body(`,"freeAllowance":{"limitMicrocredits":1000,"usedMicrocredits":300,`+
		`"remainingMicrocredits":700,"resetsAt":"2026-11-25T12:00:00Z"}`)).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 || !strings.Contains(out, "Free gen     7.00 of 10.00 credits left this cycle (3.00 used, resets 2026-11-2") ||
		!strings.Contains(out, "not clips") {
		t.Fatalf("%d %s\n%s", code, errOut, out)
	}
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, body(`,"freeAllowance":{"limitMicrocredits":0,"usedMicrocredits":0,"remainingMicrocredits":0}`)).URL)
	if out, _, _ := run(t, "", "account"); strings.Contains(out, "Free gen") {
		t.Errorf("a plan without an allowance shows one:\n%s", out)
	}
}

// The balance is one number: no Atmee V2 figure, even from a server that
// still reports one.
func TestBalanceOneKindOfCredit(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"wallet":"direct","spendableMicrocredits":4998500,"planMicrocredits":98500,"topupMicrocredits":4900000,
			"couponMicrocredits":0,"couponSpendable":true,"v2SpendableMicrocredits":120000,"heldByLiveSessionsMicrocredits":0,
			"unlimited":false,"nextExpiry":[]}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if out := ok(t, "balance"); strings.Contains(out, "Atmee V2") {
		t.Errorf("the balance shows a V2 figure:\n%s", out)
	}
}

// An account without Atmee V2 access is told it needs Pro and up, and V2
// HD below Growth that it needs Growth and up; never an invite code.
func TestAccountV2Access(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},"v2Access":"invite_required",
		"limits":{"apiAccess":true,"v2Pro":false,"videoGeneration":true},
		"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
			"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0},"captureQuota":{"pending":0,"pendingLimit":10}}`).URL)
	out := ok(t, "account")
	for _, want := range []string{"v2 avatars   can't build: Atmee V2 needs Pro and up", "Existing V2 avatars keep working", "anyone can talk to any avatar",
		"V2 HD        can't build: Atmee V2 HD needs Growth and up"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, not := range []string{"invite", "redeem", "Video clips", "credits for", "Enterprise", "help center"} {
		if strings.Contains(out, not) {
			t.Errorf("%q in:\n%s", not, out)
		}
	}
}

// Atmee V2 HD shows only where the server reports it: offered with its
// sizes (Growth and up, limits.v2Pro true), offered where Atmee V2 is on
// the account, or below Growth (Pro included), with the upgrade to Growth.
// Never an invite code, Enterprise or a help center to contact.
func TestAccountV2Pro(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	body := func(access, limits string) string {
		return `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},"v2Access":"` + access + `","limits":{"apiAccess":true` + limits + `},
			"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
				"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0},"captureQuota":{"pending":0,"pendingLimit":10},
			"links":{"billing":"https://www.atmee.ai/studio/billing","docs":"https://www.atmee.ai/docs/api","analytics":"https://www.atmee.ai/studio/analytics",
				"apiKeys":"https://www.atmee.ai/studio/api-keys","avatars":"https://www.atmee.ai/studio/manage","create":"https://www.atmee.ai/studio/create","explore":"https://www.atmee.ai/studio/explore"}}`
	}
	for _, c := range []struct {
		access, limits string
		want, not      []string
	}{
		{"granted", `,"v2Pro":true,"v2ProResolutions":[{"width":640,"height":640},{"width":864,"height":480}]`,
			[]string{"V2 HD        yes (--engine v2hd)\n", "V2 HD sizes  640x640, 864x480 (--resolution)"}, []string{"needs Growth and up", "upgrade"}},
		{"granted", `,"v2Pro":false,"v2ProResolutions":[{"width":640,"height":640}]`,
			[]string{"V2 HD        can't build: Atmee V2 HD needs Growth and up; --engine v2hd answers 403 v2hd_access_required without it. Existing V2 HD avatars keep working\n",
				"V2 HD sizes  640x640 (--resolution)"}, []string{"yes (--engine v2hd", "invite", "redeem"}},
		{"invite_required", `,"v2Pro":true`, []string{"V2 HD        yes, where Atmee V2 is on this account (--engine v2hd)"}, nil},
		{"granted", ``, nil, nil},
	} {
		t.Setenv("ATMEE_BASE_URL", accountAPI(t, body(c.access, c.limits)).URL)
		out, errOut, code := run(t, "", "account")
		if code != 0 {
			t.Fatalf("%d %s", code, errOut)
		}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s: missing %q in:\n%s", c.limits, w, out)
			}
		}
		for _, not := range append([]string{"Enterprise", "help center"}, c.not...) {
			if strings.Contains(out, not) {
				t.Errorf("%s: %q in:\n%s", c.limits, not, out)
			}
		}
		if c.want == nil && strings.Contains(out, "V2 HD") {
			t.Errorf("V2 HD shown where the server does not offer it:\n%s", out)
		}
	}
}

// The 2026 prices: a conversation rate per engine and the clip per 15
// seconds; the deprecated per-second clip price is not shown beside it.
func TestAccountPerEnginePrices(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},
		"limits":{"apiAccess":true},
		"pricing":{"sessionPerMinuteMicrocredits":100,"sessionV2PerMinuteMicrocredits":200,"monetizedPerMinuteMicrocredits":200,
			"monetizedV2PerMinuteMicrocredits":400,"renderV1PerMinuteMicrocredits":50,"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0,
			"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100,
				"clipPer15SecondsMicrocredits":100,"clipPerSecondMicrocredits":7}},
		"captureQuota":{"pending":0,"pendingLimit":10}}`).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"conversation v1 1.00/min, v2 2.00/min · avatar session v1 0.50/min, v2 1.50/min",
		"voice 1.00 · clip 1.00 per 15 s credits"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/s ") {
		t.Errorf("per-second clip price shown:\n%s", out)
	}
}

// Atmee V2 HD's own rates show where the server reports them; where it
// does not, its price is left unsaid (not taken for v2's), and --json leaves
// the fields out.
func TestAccountV2HDRates(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	body := func(hd string) string {
		return `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},"v2Access":"granted",
			"limits":{"apiAccess":true,"v2Pro":true},
			"pricing":{"sessionPerMinuteMicrocredits":100,"sessionV2PerMinuteMicrocredits":200,"monetizedPerMinuteMicrocredits":200,
				"monetizedV2PerMinuteMicrocredits":300,"renderV1PerMinuteMicrocredits":50,"renderV2PerMinuteMicrocredits":150,
				"previewClipMicrocredits":0` + hd + `},
			"captureQuota":{"pending":0,"pendingLimit":10}}`
	}
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, body(`,"sessionV2HdPerMinuteMicrocredits":300,"renderV2HdPerMinuteMicrocredits":200,"monetizedV2HdPerMinuteMicrocredits":400`)).URL)
	out := ok(t, "account")
	if !strings.Contains(out, "conversation v1 1.00/min, v2 2.00/min, V2 HD 3.00/min · avatar session v1 0.50/min, v2 1.50/min, V2 HD 2.00/min\n") ||
		strings.Contains(out, "v2's rates") {
		t.Errorf("reported:\n%s", out)
	}
	if js := ok(t, "account", "--json"); !strings.Contains(js, `"monetizedV2HdPerMinuteMicrocredits": 400`) {
		t.Errorf("--json: %s", js)
	}
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, body("")).URL)
	out = ok(t, "account")
	if !strings.Contains(out, "conversation v1 1.00/min, v2 2.00/min · avatar session v1 0.50/min, v2 1.50/min\n") ||
		strings.Contains(out, "rates") || !strings.Contains(out, "V2 HD        yes (--engine v2hd)\n") {
		t.Errorf("not reported:\n%s", out)
	}
	if js := ok(t, "account", "--json"); strings.Contains(js, "V2Hd") {
		t.Errorf("--json shows absent V2 HD rates: %s", js)
	}
}

// A reported v2 rate of 0 is shown as 0, not taken for an older server.
func TestAccountFreeV2Conversation(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},
		"limits":{"apiAccess":true},
		"pricing":{"sessionPerMinuteMicrocredits":100,"sessionV2PerMinuteMicrocredits":0,"monetizedPerMinuteMicrocredits":200,
			"monetizedV2PerMinuteMicrocredits":300,"renderV1PerMinuteMicrocredits":50,"renderV2PerMinuteMicrocredits":150,"previewClipMicrocredits":0},
		"captureQuota":{"pending":0,"pendingLimit":10}}`).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 || !strings.Contains(out, "conversation v1 1.00/min, v2 0.00/min") {
		t.Fatalf("%d %s\n%s", code, errOut, out)
	}
	out, _, _ = run(t, "", "account", "--json")
	if !strings.Contains(out, `"sessionV2PerMinuteMicrocredits": 0`) || strings.Contains(out, "SessionV2Reported") {
		t.Fatalf("--json: %s", out)
	}
}

func TestWhoami(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t).URL)
	out, errOut, code := run(t, "", "whoami")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Key          prod backend  k-1", "Scopes       sessions, avatars:read", "Avatars      any avatar of the account",
		"Spend cap    12.50 of 500.00 credits used this month, 487.50 left (resets 2026-11-0", "Key limits   sessions up to 10m0s"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	out, _, _ = run(t, "", "whoami", "--json")
	var w map[string]any
	if err := json.Unmarshal([]byte(out), &w); err != nil || w["api_key"] != true || w["key"].(map[string]any)["name"] != "prod backend" {
		t.Fatalf("--json: %v %s", err, out)
	}
}

// Offline (or refused), whoami still shows the saved key and the base URL.
func TestWhoamiOffline(t *testing.T) {
	isolate(t) // ATMEE_BASE_URL points nowhere
	t.Setenv("ATMEE_API_KEY", "sk_atmee_0123456789abcdef")
	out, errOut, code := run(t, "", "whoami")
	if code != 0 || !strings.Contains(out, "API key sk_atmee_…cdef -> http://127.0.0.1:1") || !strings.Contains(errOut, "offline or unreachable") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	out, _, code = run(t, "", "whoami", "--json")
	var w map[string]any
	if err := json.Unmarshal([]byte(out), &w); err != nil || code != 0 || w["api_key"] != true || w["key_error"] == nil || w["key"] != nil {
		t.Fatalf("--json: %v %s", err, out)
	}
	// A key the API refuses: the answer is shown, not a crash.
	t.Setenv("ATMEE_BASE_URL", accountAPI(t).URL)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_revoked0000")
	if out, errOut, code := run(t, "", "whoami"); code != 0 || !strings.Contains(out, "API key") || !strings.Contains(errOut, "401") {
		t.Fatalf("refused: %d %q %q", code, out, errOut)
	}
}

func TestBalanceNeedsAKey(t *testing.T) {
	isolate(t)
	if _, errOut, code := run(t, "", "balance"); code != 1 || !strings.Contains(errOut, "atmee login") {
		t.Fatalf("%d %s", code, errOut)
	}
}

// redeem sends the code to POST /v1/account/redeem and says what it gave;
// the API's refusal (an API key: login_required) is the error.
func TestRedeem(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var body map[string]any
	status, answer := http.StatusOK, `{"microcreditsGranted":5000,"v2Access":"granted"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/account/redeem" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)

	out, errOut, code := run(t, "", "redeem", "V2-LAUNCH")
	if code != 0 || body["code"] != "V2-LAUNCH" {
		t.Fatalf("redeem: %d %s (sent %v)", code, errOut, body)
	}
	if !strings.Contains(out, "50.00 credits added") || !strings.Contains(out, "this account can build Atmee V2") {
		t.Errorf("output: %q", out)
	}
	if out, _, _ := run(t, "", "redeem", "V2-LAUNCH", "--json"); !strings.Contains(out, `"microcreditsGranted": 5000`) {
		t.Errorf("--json: %q", out)
	}

	// A V2 HD code (or a Growth account) says V2 HD can be built too.
	answer = `{"microcreditsGranted":0,"v2Access":"granted","v2Pro":true}`
	if out, _, _ := run(t, "", "redeem", "HD-CODE"); !strings.Contains(out, "this account can build Atmee V2 and V2 HD") {
		t.Errorf("v2hd code: %q", out)
	}
	// v2Pro without V2 access never claims V2 HD.
	answer = `{"microcreditsGranted":5000,"v2Access":"invite_required","v2Pro":true}`
	if out, _, _ := run(t, "", "redeem", "ODD"); strings.Contains(out, "V2 HD") || !strings.Contains(out, "did not unlock building Atmee V2") {
		t.Errorf("v2Pro without v2 access: %q", out)
	}
	// v2Pro false leaves the V2 line as it was.
	answer = `{"microcreditsGranted":0,"v2Access":"granted","v2Pro":false}`
	if out, _, _ := run(t, "", "redeem", "V2-CODE"); !strings.Contains(out, "this account can build Atmee V2") || strings.Contains(out, "V2 HD") {
		t.Errorf("v2 code: %q", out)
	}

	// A credits-only code gives no v2 access, and says how v2 opens.
	answer = `{"microcreditsGranted":5000,"v2Access":"invite_required"}`
	if out, _, _ := run(t, "", "redeem", "V2-LAUNCH"); !strings.Contains(out, "did not unlock building Atmee V2") || !strings.Contains(out, "Pro and up") ||
		strings.Contains(out, "invite") {
		t.Errorf("credits-only code: %q", out)
	}

	status, answer = http.StatusForbidden, `{"error":"login_required","message":"codes are redeemed by a signed-in user"}`
	if _, errOut, code := run(t, "", "redeem", "V2-LAUNCH"); code == 0 || !strings.Contains(errOut, "login_required") {
		t.Errorf("API key: %d %q", code, errOut)
	}
}

// redeem still works but is hidden: the help never lists it.
func TestRedeemHidden(t *testing.T) {
	isolate(t)
	out, errOut, code := run(t, "", "--help")
	if code != 0 || strings.Contains(out, "redeem") {
		t.Errorf("help lists redeem: %d %s%s", code, out, errOut)
	}
}
