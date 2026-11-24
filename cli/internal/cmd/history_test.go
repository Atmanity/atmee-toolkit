package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const historyReq = "5b0c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f"

// historyAPI answers the history routes with canned pages and records each
// request as METHOD path?query.
func historyAPI(t *testing.T) *[]string {
	t.Helper()
	var reqs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/generation" && r.URL.Query().Get("cursor") == "":
			_, _ = w.Write([]byte(`{"items":[{"requestId":"` + historyReq + `","kind":"clip","operation":"clip","status":"ready",
				"createdAt":"2026-11-20T00:00:00Z","deletedAt":null,"expiresAt":"2026-12-20T00:00:00Z",
				"madeBy":{"type":"member","name":"ada","avatarUrl":null},"microcreditsCharged":30,
				"inputs":{"mode":"picture","prompt":"she walks along the beach"},
				"remake":{"possible":true,"imageUploadRequired":true,"audioUploadRequired":false},
				"outputs":[{"uploadId":"aaaaaaaa-0000-4000-8000-000000000001","status":"ready","expiresAt":"2026-12-20T00:00:00Z","url":"https://x/clip.mp4"}]}],
				"nextCursor":"c2"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/generation":
			_, _ = w.Write([]byte(`{"items":[{"requestId":"6b0c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f","kind":"voice","operation":"voice_design","status":"expired",
				"createdAt":"2026-11-01T00:00:00Z","deletedAt":null,"expiresAt":"2026-11-08T00:00:00Z","madeBy":{"type":"apiKey","name":"bot","avatarUrl":null,"revoked":true},
				"microcreditsCharged":null,"inputs":null,"remake":{"possible":false,"imageUploadRequired":false,"audioUploadRequired":false},
				"outputs":[{"uploadId":"aaaaaaaa-0000-4000-8000-000000000002","status":"expired","expiresAt":"2026-11-08T00:00:00Z"}]}],"nextCursor":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/generation/summary":
			_, _ = w.Write([]byte(`{"total":3,"counts":{"clip":2,"portrait":0,"voice":1},"clipModes":{"picture":1,"avatar":1,"unknown":0},
				"makers":[{"generations":2,"lastAt":"2026-11-20T00:00:00Z","madeBy":{"type":"member","name":"ada","avatarUrl":null,"userId":"bbbbbbbb-0000-4000-8000-000000000001"}}],
				"unknownGenerations":1}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/generation/"+historyReq:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	return &reqs
}

// list sends its filters as the API's query, prints each generation and
// the cursor for the next page; --all follows the cursor to the end.
func TestGenerateListFiltersAndPages(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	reqs := historyAPI(t)
	out, errOut, code := run(t, "", "generate", "list", "--kind", "clip", "--mode", "picture,avatar", "--status", "ready", "--made-by", "me", "-q", "beach", "--limit", "5")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	q, _ := url.ParseQuery(strings.SplitN((*reqs)[0], "?", 2)[1])
	if q.Get("kind") != "clip" || q.Get("status") != "ready" || q.Get("madeBy") != "me" || q.Get("q") != "beach" || q.Get("limit") != "5" || len(q["mode"]) == 0 {
		t.Fatalf("query %v", q)
	}
	for _, want := range []string{historyReq, "clip (picture)", "by ada · 0.30 credits", "she walks along the beach", "https://x/clip.mp4", "--cursor c2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	*reqs = nil
	out, _, code = run(t, "", "generate", "list", "--all")
	if code != 0 || len(*reqs) != 2 || !strings.Contains(out, "API key bot (revoked)") || !strings.Contains(out, "charge unknown") || strings.Contains(out, "--cursor") {
		t.Fatalf("%d %v\n%s", code, *reqs, out)
	}
}

// Bad filters are refused before any request.
func TestGenerateListRefusesBadFilters(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	reqs := historyAPI(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--kind", "video"}, "--kind takes clip, portrait or voice"},
		{[]string{"--mode", "photo"}, "--mode takes picture, avatar or unknown"},
		{[]string{"--kind", "voice", "--mode", "avatar"}, "--mode is for clips"},
		{[]string{"--status", "done"}, "--status takes"},
		{[]string{"--since", "yesterday"}, "--since takes"},
		{[]string{"--limit", "51"}, "--limit takes 1 to 50"},
		{[]string{"--all", "--cursor", "c2"}, "leave out --cursor"},
	} {
		_, errOut, code := run(t, "", append([]string{"generate", "list"}, c.args...)...)
		if code != 2 || !strings.Contains(errOut, c.want) {
			t.Errorf("%v: %d %s", c.args, code, errOut)
		}
	}
	if len(*reqs) != 0 {
		t.Fatalf("sent %v", *reqs)
	}
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"7d":                   now.AddDate(0, 0, -7),
		"72h":                  now.Add(-72 * time.Hour),
		"2026-11-01T00:00:00Z": time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	} {
		got, err := parseSince(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("%s: %v %v", in, got, err)
		}
	}
}

func TestGenerateSummary(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	historyAPI(t)
	out, errOut, code := run(t, "", "generate", "summary")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"3 generations: 2 clips (1 picture, 1 avatar, 0 unknown)", "ada  bbbbbbbb-0000-4000-8000-000000000001", "1 generation with no recorded maker"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

// delete asks first (--yes skips it), checks the id and operation before
// any request, and passes --operation on.
func TestGenerateDelete(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	reqs := historyAPI(t)
	if _, errOut, code := run(t, "", "generate", "delete", "nope", "--yes"); code != 2 || !strings.Contains(errOut, "not a generation's request id") {
		t.Fatalf("%d %s", code, errOut)
	}
	if _, errOut, code := run(t, "", "generate", "delete", historyReq, "--operation", "video", "--yes"); code != 2 || !strings.Contains(errOut, "--operation takes") {
		t.Fatalf("%d %s", code, errOut)
	}
	if _, _, code := run(t, "n\n", "generate", "delete", historyReq); code == 0 {
		t.Fatal("deleted without a yes")
	}
	if len(*reqs) != 0 {
		t.Fatalf("sent %v", *reqs)
	}
	out, errOut, code := run(t, "", "generate", "delete", historyReq, "--operation", "clip", "--yes")
	if code != 0 || !strings.Contains(out, "Deleted the outputs of "+historyReq) || len(*reqs) != 1 || !strings.Contains((*reqs)[0], "operation=clip") {
		t.Fatalf("%d %s %s %v", code, out, errOut, *reqs)
	}
}
