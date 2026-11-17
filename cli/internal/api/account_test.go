package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// limits.v2Trial's days and limits.v2Unlock decode into their types, and
// go back to JSON as the server sent them: an older server's trial without
// days gets no days of 0, and an unlock (or its null) is kept though the
// generated limits do not know it.
func TestAccountV2TrialAndUnlock(t *testing.T) {
	const base = `{"accountId":"11111111-1111-4111-8111-111111111111","plan":{},"pricing":{"sessionV2PerMinuteMicrocredits":1,"monetizedV2PerMinuteMicrocredits":1},
		"captureQuota":{"pending":0,"pendingLimit":1},"limits":{"apiAccess":true%s}}`
	for _, c := range []struct {
		limits       string
		days         int // -1: absent
		unlock       string
		keep, absent []string
	}{
		{limits: ``, days: -1, absent: []string{`"v2Trial"`, `"v2Unlock"`}},
		{limits: `,"v2Trial":{"state":"active"}`, days: -1, absent: []string{`"days"`, `"v2Unlock"`}},
		{limits: `,"v2Trial":{"state":"ended","days":0},"v2Unlock":null`, days: 0, keep: []string{`"days":0`, `"v2Unlock":null`}},
		{limits: `,"v2Unlock":{"level":"v2hd","source":"coupon","expiresAt":"2026-12-31T12:00:00Z"}`, days: -1, unlock: "v2hd",
			keep: []string{`"v2Unlock":{"level":"v2hd","source":"coupon","expiresAt":"2026-12-31T12:00:00Z"}`}},
		{limits: `,"v2Unlock":{"level":"v2","source":"legacy","expiresAt":null}`, days: -1, unlock: "v2", keep: []string{`"expiresAt":null`}},
	} {
		var a Account
		if err := json.Unmarshal([]byte(strings.Replace(base, "%s", c.limits, 1)), &a); err != nil {
			t.Fatalf("%s: %v", c.limits, err)
		}
		switch {
		case c.days < 0 && a.V2Trial != nil && a.V2Trial.Days != nil:
			t.Errorf("%s: days %d, want absent", c.limits, *a.V2Trial.Days)
		case c.days >= 0 && (a.V2Trial == nil || a.V2Trial.Days == nil || *a.V2Trial.Days != c.days):
			t.Errorf("%s: trial %+v, want days %d", c.limits, a.V2Trial, c.days)
		}
		got := ""
		if a.Limits.V2Unlock != nil {
			got = string(a.Limits.V2Unlock.Level)
		}
		if got != c.unlock {
			t.Errorf("%s: unlock level %q, want %q", c.limits, got, c.unlock)
		}
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range c.keep {
			if !strings.Contains(string(b), k) {
				t.Errorf("%s: JSON lacks %s: %s", c.limits, k, b)
			}
		}
		for _, k := range c.absent {
			if strings.Contains(string(b), k) {
				t.Errorf("%s: JSON has %s: %s", c.limits, k, b)
			}
		}
	}
}

// A redemption's v2Unlock goes back to JSON as the server sent it: left out
// by an older server, null or an unlock by a current one.
func TestRedemptionV2Unlock(t *testing.T) {
	for in, want := range map[string]string{
		`{"microcreditsGranted":100}`:               "",
		`{"microcreditsGranted":0,"v2Unlock":null}`: `"v2Unlock":null`,
		`{"microcreditsGranted":0,"v2Unlock":{"level":"v2","source":"coupon","expiresAt":"2026-12-31T12:00:00Z"}}`: `"v2Unlock":{"expiresAt":"2026-12-31T12:00:00Z","level":"v2","source":"coupon"}`,
	} {
		var r Redemption
		if err := json.Unmarshal([]byte(in), &r); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" && strings.Contains(string(b), "v2Unlock") || want != "" && !strings.Contains(string(b), want) {
			t.Errorf("%s: JSON %s, want %q", in, b, want)
		}
	}
}
