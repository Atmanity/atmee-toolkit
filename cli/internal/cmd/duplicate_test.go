package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const dupSource = "11111111-2222-4333-8444-555555555555"

// duplicateAPI answers POST /v1/avatars/{id}/duplicate with status and
// payload, and records each request's path and body.
func duplicateAPI(t *testing.T, status int, payload string, paths *[]string, bodies *[]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*paths = append(*paths, r.Method+" "+r.URL.Path)
		*bodies = append(*bodies, string(b))
		if strings.HasPrefix(payload, "{") {
			w.Header().Set("Content-Type", "application/json")
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		if status == 202 {
			w.Header().Set("X-RateLimit-Limit", "10")
			w.Header().Set("X-RateLimit-Remaining", "9")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
}

func TestDuplicateSendsOnlyWhatWasGiven(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	copyID := "99999999-2222-4333-8444-555555555555"
	var paths, bodies []string
	duplicateAPI(t, 202, `{"avatarId":"`+copyID+`","kind":"conversational","status":"building","statusUrl":"x","engine":"v2"}`, &paths, &bodies)

	out, errOut, code := run(t, "", "avatars", "duplicate", dupSource)
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if paths[0] != "POST /v1/avatars/"+dupSource+"/duplicate" || strings.TrimSpace(bodies[0]) != "{}" {
		t.Errorf("request: %v %v", paths, bodies)
	}
	if !strings.Contains(out, "Created "+copyID+" from "+dupSource+" (v2, building)") || !strings.Contains(errOut, "Room for 9 more captures") {
		t.Errorf("output: %q %q", out, errOut)
	}

	// --engine v2hd goes out as v2pro, the name every server accepts.
	_, errOut, code = run(t, "", "avatars", "duplicate", dupSource, "--name", " Ada (V2) ", "--engine", "v2hd")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	var got map[string]any
	_ = json.Unmarshal([]byte(bodies[1]), &got)
	if got["name"] != "Ada (V2)" || got["engine"] != "v2pro" {
		t.Errorf("body: %s", bodies[1])
	}

	out, _, code = run(t, "", "avatars", "duplicate", dupSource, "--json")
	if code != 0 || !strings.Contains(out, `"avatarId": "`+copyID+`"`) {
		t.Errorf("--json: %d %s", code, out)
	}
}

func TestDuplicateDraftSaysWhatItNeeds(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	copyID := "99999999-2222-4333-8444-555555555555"
	var paths, bodies []string
	duplicateAPI(t, 202, `{"avatarId":"`+copyID+`","kind":"conversational","status":"draft","missingInputs":["voice"],"statusUrl":"x","engine":"v2"}`, &paths, &bodies)
	out, errOut, code := run(t, "", "avatars", "duplicate", dupSource, "--engine", "v2")
	if code != 0 || !strings.Contains(out, "(v2, draft)") || !strings.Contains(out, "replace-voice "+copyID) {
		t.Errorf("%d %q %q", code, out, errOut)
	}
}

func TestDuplicateUsageErrors(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", "http://127.0.0.1:1") // never reached
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"avatars", "duplicate"}, "missing AVATAR_ID"},
		{[]string{"avatars", "duplicate", "not-a-uuid"}, "not an avatar or session id"},
		{[]string{"avatars", "duplicate", dupSource, "--engine", "v3"}, "--engine takes v1, v2 or v2hd"},
		{[]string{"avatars", "duplicate", dupSource, "--name", " "}, "--name takes 1-100 characters"},
		{[]string{"avatars", "duplicate", dupSource, "--name", strings.Repeat("a", 101)}, "--name takes 1-100 characters"},
	} {
		if _, errOut, code := run(t, "", c.args...); code != 2 || !strings.Contains(errOut, c.want) {
			t.Errorf("%v: %d %s", c.args, code, errOut)
		}
	}
}

func TestDuplicateErrors(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	for _, c := range []struct {
		status  int
		payload string
		want    []string
	}{
		{404, `{"error":"avatar_not_found","message":"avatar not found"}`, []string{"404 avatar_not_found"}},
		// A server from before duplication: the route itself is missing.
		{404, `404 page not found`, []string{"cannot duplicate avatars yet"}},
		{409, `{"error":"source_file_missing","message":"the portrait is no longer in storage; replace it on the source avatar and try again"}`,
			[]string{"409 source_file_missing", "replace it on the source"}},
		{402, `{"error":"insufficient_v2_credits","message":"Atmee V2 is not on this account's plan"}`, []string{"upgrade to Pro and up", "--engine v1"}},
		{403, `{"error":"v2_pro_not_in_plan","message":"no V2 HD"}`, []string{"no V2 HD", "--engine v2"}},
	} {
		var paths, bodies []string
		duplicateAPI(t, c.status, c.payload, &paths, &bodies)
		_, errOut, code := run(t, "", "avatars", "duplicate", dupSource, "--engine", "v2")
		if code != 1 || len(paths) != 1 {
			t.Errorf("%d: code %d, %d requests, %s", c.status, code, len(paths), errOut)
		}
		for _, w := range c.want {
			if !strings.Contains(errOut, w) {
				t.Errorf("%d: %q lacks %q", c.status, errOut, w)
			}
		}
		if c.payload[0] == '{' && strings.Contains(errOut, "cannot duplicate avatars yet") {
			t.Errorf("%d: %q blames the server version", c.status, errOut)
		}
	}
}
