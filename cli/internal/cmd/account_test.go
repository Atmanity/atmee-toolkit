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
			"plan":{"code":"scale","name":"Scale","includedMicrocredits":100000,"cycleEnd":"2026-11-01T12:00:00Z","pendingPlan":{"code":"pro","name":"Pro"}},
			"limits":{"apiAccess":true,"maxConcurrentSessions":7,"maxApiAvatars":50},
			"pricing":{"sessionPerMinuteMicrocredits":100,"monetizedPerMinuteMicrocredits":200,"renderV1PerMinuteMicrocredits":50,
				"renderV2PerMinuteMicrocredits":100,"previewClipMicrocredits":0,
				"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100,"clipPerSecondMicrocredits":10,
					"clipResolutions":[{"width":512,"height":512},{"width":384,"height":672}]}},
			"captureQuota":{"pending":2,"pendingLimit":10},"payouts":{"status":"verified","verifiedAt":"2026-09-01T12:00:00Z"}}`,
		"/v1/api_keys/current": `{"kind":"api_key","id":"k-1","accountId":"11111111-1111-4111-8111-111111111111","name":"prod backend",
			"scopes":["sessions","avatars:read"],"billingMode":"metered","active":true,"maxSessionSeconds":600,
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
		"Next cycle   Pro (pro)", "API access   on", "session length no plan cap · 7 at once · 50 API avatars",
		"conversation 1.00/min · avatar session v1 0.50/min, v2 1.00/min", "Generation   portraits 1.00 · enhance 1.00 · voice 1.00 · clip 0.10/s credits", "Preview clip 0.00 credits", "v2 sizes     512x512, 384x672", "Captures     2 of 10 queued or running", "Payouts      verified since 2026-09-0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	out, _, _ = run(t, "", "account", "--json")
	if !strings.Contains(out, `"renderV2PerMinuteMicrocredits": 100`) || !strings.Contains(out, `"status": "verified"`) ||
		strings.Contains(out, "V2PerMinuteMicrocredits\": 0") {
		t.Fatalf("--json: %s", out)
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
			"monetizedV2PerMinuteMicrocredits":400,"renderV1PerMinuteMicrocredits":50,"renderV2PerMinuteMicrocredits":100,"previewClipMicrocredits":0,
			"generation":{"portraitsMicrocredits":100,"enhancementsMicrocredits":100,"voicesMicrocredits":100,
				"clipPer15SecondsMicrocredits":100,"clipPerSecondMicrocredits":7}},
		"captureQuota":{"pending":0,"pendingLimit":10}}`).URL)
	out, errOut, code := run(t, "", "account")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"conversation v1 1.00/min, v2 2.00/min · avatar session v1 0.50/min, v2 1.00/min",
		"voice 1.00 · clip 1.00 per 15 s credits"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/s ") {
		t.Errorf("per-second clip price shown:\n%s", out)
	}
}

// A reported v2 rate of 0 is shown as 0, not taken for an older server.
func TestAccountFreeV2Conversation(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t, `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},
		"limits":{"apiAccess":true},
		"pricing":{"sessionPerMinuteMicrocredits":100,"sessionV2PerMinuteMicrocredits":0,"monetizedPerMinuteMicrocredits":200,
			"monetizedV2PerMinuteMicrocredits":300,"renderV1PerMinuteMicrocredits":50,"renderV2PerMinuteMicrocredits":100,"previewClipMicrocredits":0},
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
		"Billing      metered", "Spend cap    12.50 of 500.00 credits used this month, 487.50 left (resets 2026-11-0", "Key limits   sessions up to 10m0s"} {
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
	if !strings.Contains(out, "50.00 credits added") || !strings.Contains(out, "v2 avatars are open") {
		t.Errorf("output: %q", out)
	}
	if out, _, _ := run(t, "", "redeem", "V2-LAUNCH", "--json"); !strings.Contains(out, `"microcreditsGranted": 5000`) {
		t.Errorf("--json: %q", out)
	}

	status, answer = http.StatusForbidden, `{"error":"login_required","message":"codes are redeemed by a signed-in user"}`
	if _, errOut, code := run(t, "", "redeem", "V2-LAUNCH"); code == 0 || !strings.Contains(errOut, "login_required") {
		t.Errorf("API key: %d %q", code, errOut)
	}
}
