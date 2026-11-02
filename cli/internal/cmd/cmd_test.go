package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Atmanity/toolkit/cli/internal/web"
)

// run executes the CLI with stdin and returns stdout, stderr and the code.
func run(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Execute(args, &out, &errOut, strings.NewReader(stdin))
	return out.String(), errOut.String(), code
}

func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("ATMEE_HOME", home)
	t.Setenv("ATMEE_API_KEY", "")
	// Nothing here reaches a real API: a test that needs one sets its own.
	t.Setenv("ATMEE_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("ATMEE_NO_BROWSER", "1")
	return home
}

func TestLoginWithAPIKeyAndLogout(t *testing.T) {
	isolate(t)
	if _, errOut, code := run(t, "not-a-key\n", "login", "--api-key"); code != 1 || !strings.Contains(errOut, "does not look like") {
		t.Fatalf("bad key: %d %s", code, errOut)
	}
	out, _, code := run(t, "sk_atmee_0123456789abcdef\n", "login", "--api-key")
	if code != 0 || !strings.Contains(out, "sk_atmee_…cdef") || strings.Contains(out, "0123456789") {
		t.Fatalf("save: %d %q", code, out)
	}
	out, _, _ = run(t, "", "whoami", "--json")
	var w map[string]any
	_ = json.Unmarshal([]byte(out), &w)
	if w["api_key"] != true {
		t.Fatalf("whoami: %s", out)
	}
	run(t, "", "logout")
	if out, _, _ = run(t, "", "whoami", "--json"); !strings.Contains(out, `"api_key": false`) {
		t.Fatalf("logout removes the key: %s", out)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	for _, args := range [][]string{
		{"avatars", "create", "--image", "x"},                         // --name missing
		{"avatars", "get"},                                            // id missing
		{"avatars", "update", "11111111-2222-4333-8444-555555555555"}, // no field
		{"avatars", "frobnicate"},
	} {
		if _, errOut, code := run(t, "", args...); code != 2 {
			t.Errorf("%v: code %d %s", args, code, errOut)
		}
	}
}

func TestNoAPIKeyExplainsHowToGetOne(t *testing.T) {
	isolate(t)
	if _, errOut, code := run(t, "", "avatars", "list"); code != 1 || !strings.Contains(errOut, "atmee login") {
		t.Fatalf("%d %s", code, errOut)
	}
}

func TestDeleteAsksFirst(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", "http://127.0.0.1:1") // never reached
	if _, errOut, code := run(t, "n\n", "avatars", "delete", "11111111-2222-4333-8444-555555555555"); code != 2 || !strings.Contains(errOut, "aborted") {
		t.Fatalf("%d %s", code, errOut)
	}
}

func TestConfigMustBeAJSONObject(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	for _, body := range []string{"[1,2]", "{nope", `{"imgae": "https://x/p.png"}`} {
		if _, errOut, code := run(t, body, "avatars", "create", "--name", "X", "--config", "-"); code != 2 {
			t.Errorf("%s: %d %s", body, code, errOut)
		}
	}
}

func TestUpdateClearAndEmptyValues(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", "http://127.0.0.1:1") // never reached
	id := "11111111-2222-4333-8444-555555555555"
	for _, args := range [][]string{
		{"avatars", "update", id, "--language", ""},
		{"avatars", "update", id, "--clear", "name"},
		{"avatars", "update", id, "--greeting", "Hi", "--clear", "greeting"},
		{"avatars", "update", id, "--clear", "language"},   // required: cannot be removed
		{"avatars", "update", id, "--clear", "self-intro"}, // the API refuses an empty self-introduction
		{"avatars", "update", id, "--self-intro", " "},
		{"avatars", "update", id, "--guardrail", "Be kind", "--clear", "guardrails"},
		{"avatars", "update", id, "--facts", " "},
		{"avatars", "update", id, "--facts", "Basil.", "--clear", "facts"},
	} {
		if _, errOut, code := run(t, "", args...); code != 2 {
			t.Errorf("%v: %d %s", args, code, errOut)
		}
	}
}

func TestUpdateClearSendsEmptyValues(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	args := []string{"avatars", "update", id, "--json"}
	for _, f := range []string{"description", "greeting", "alias", "preview-message", "guardrails", "facts"} {
		args = append(args, "--clear", f)
	}
	if _, errOut, code := run(t, "", args...); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	// null would leave the values unchanged: removing one means sending it empty.
	want := map[string]any{"description": "", "greeting": "", "preferredAlias": "", "previewMessage": "", "guardrails": []any{},
		"knowledge": map[string]any{"facts": ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body: %v", got)
	}
}

// postAsPage posts like the served page does: with the secret it carries.
func TestLoginAPIKeyJSON(t *testing.T) {
	isolate(t)
	out, _, code := run(t, "sk_atmee_0123456789abcdef\n", "login", "--api-key", "--json")
	var m map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &m) != nil || m["api_key"] != true {
		t.Fatalf("%d %q", code, out)
	}
}

func postAsPage(url, body string) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	m := regexp.MustCompile(`'X-Atmee-Secret': ?'([0-9a-f]+)'`).FindStringSubmatch(string(b))
	if m == nil {
		return
	}
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Atmee-Secret", m[1])
	if r, err := http.DefaultClient.Do(req); err == nil {
		r.Body.Close()
	}
}

func TestGrantAmountMustBeAWholeNumber(t *testing.T) {
	isolate(t)
	for _, n := range []string{"1000abc", "1e3", "0"} {
		if _, errOut, code := run(t, "", "avatars", "sponsor", "grant", "11111111-2222-4333-8444-555555555555", n); code != 2 || !strings.Contains(errOut, "positive whole number") {
			t.Errorf("%s: %d %s", n, code, errOut)
		}
	}
}

// avatarServer answers every request with the avatar detail body.
func avatarServer(t *testing.T, body string) {
	t.Helper()
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWaitJSONReportsAFailedWrite(t *testing.T) {
	id := "11111111-2222-4333-8444-555555555555"
	avatarServer(t, `{"id":"`+id+`","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z"}`)
	var errOut bytes.Buffer
	if code := Execute([]string{"avatars", "wait", id, "--json"}, failingWriter{}, &errOut, strings.NewReader("")); code != 1 || !strings.Contains(errOut.String(), "disk full") {
		t.Fatalf("%d %s", code, errOut.String())
	}
}

func TestSelectTakesAnyCandidateIDSpellingAndSeconds(t *testing.T) {
	id, pick := "11111111-2222-4333-8444-555555555555", "7e8e5254-0000-4000-8000-000000000001"
	avatarServer(t, `{"id":"`+id+`","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z",
		"engine":"v2","servingCandidateId":"`+pick+`","selectedCandidateId":"`+pick+`"}`)
	if _, errOut, code := run(t, "", "avatars", "candidates", "select", id, strings.ToUpper(pick), "--timeout", "30", "--json"); code != 0 {
		t.Fatalf("upper-case id, --timeout in seconds: %d %s", code, errOut)
	}
	if _, errOut, code := run(t, "", "avatars", "candidates", "select", id, "c-1"); code != 2 || !strings.Contains(errOut, "not a candidate id") {
		t.Fatalf("a bad candidate id is a usage error: %d %s", code, errOut)
	}
}

// "Try more" waits on its own captures only: one another client adds while
// the picker is open, which never finishes here, is not waited on.
func TestPickMoreWaitsOnlyOnItsOwnCaptures(t *testing.T) {
	id := "11111111-2222-4333-8444-555555555555"
	first, other, mine := "7e8e5254-0000-4000-8000-000000000001", "7e8e5254-0000-4000-8000-000000000002", "7e8e5254-0000-4000-8000-000000000003"
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var mu sync.Mutex
	type cand struct{ id, status string }
	cands := []cand{{first, "completed"}}
	selected := first
	list := func() []byte {
		var items []map[string]any
		for _, c := range cands {
			x := map[string]any{"candidateId": c.id, "generation": 1, "status": c.status, "seed": 1,
				"serving": c.id == selected, "createdAt": "2026-10-16T00:00:00Z"}
			if c.status == "completed" {
				x["previewUrl"] = "https://example.com/" + c.id + ".mp4"
			}
			items = append(items, x)
		}
		b, _ := json.Marshal(map[string]any{"generation": 1, "items": items, "rebuilding": false,
			"seedSelection": "manual", "selectionRequired": false})
		return b
	}
	avatar := func() []byte {
		return []byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private",
			"createdAt":"2026-10-16T00:00:00Z","engine":"v2","servingCandidateId":"` + selected + `","selectedCandidateId":"` + selected + `"}`)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/candidates") && r.Method == http.MethodPost:
			cands = append(cands, cand{mine, "completed"})
			w.WriteHeader(202)
			_, _ = w.Write(list())
		case strings.HasSuffix(r.URL.Path, "/candidates"):
			_, _ = w.Write(list())
		case strings.HasSuffix(r.URL.Path, "/selected_candidate"):
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			selected = b["candidateId"]
			_, _ = w.Write(avatar())
		default:
			_, _ = w.Write(avatar())
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	prevCapture, prevPoll, prevOpen := captureTimeout, pollInterval, web.Open
	captureTimeout, pollInterval = time.Second, 10*time.Millisecond
	t.Cleanup(func() { captureTimeout, pollInterval, web.Open = prevCapture, prevPoll, prevOpen })
	opens := 0
	web.Open = func(url string) {
		opens++
		choice := `{"more":true}`
		if opens == 1 {
			mu.Lock()
			cands = append(cands, cand{other, "processing"}) // added elsewhere while the page is open
			mu.Unlock()
		} else {
			choice = `{"selected":"` + mine + `"}`
		}
		go postAsPage(url, choice)
	}
	out, errOut, code := run(t, "", "avatars", "candidates", "pick", id, "--more", "1")
	if code != 0 || opens != 2 || !strings.Contains(out, "Serving candidate: "+mine) {
		t.Fatalf("code %d, opens %d\n%s\n%s", code, opens, out, errOut)
	}
}

// select --json prints the avatar's state when the serving wait fails, as
// avatars wait --json does.
func TestSelectJSONPrintsTheStateWhenTheWaitFails(t *testing.T) {
	id, pick := "11111111-2222-4333-8444-555555555555", "7e8e5254-0000-4000-8000-000000000001"
	avatarServer(t, `{"id":"`+id+`","name":"M","kind":"conversational","status":"failed","readyToChat":false,"visibility":"private","createdAt":"2026-10-16T00:00:00Z",
		"engine":"v2","selectedCandidateId":"`+pick+`"}`)
	out, errOut, code := run(t, "", "avatars", "candidates", "select", id, pick, "--json")
	var m map[string]any
	if code != 1 || json.Unmarshal([]byte(out), &m) != nil || m["status"] != "failed" {
		t.Fatalf("code %d, stdout %q\n%s", code, out, errOut)
	}
}

func TestOnlySponsorGrantsTakeAnIdempotencyKey(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{{"avatars", "create"}, {"avatars", "candidates", "add"}} {
		out, _, _ := run(t, "", append(args, "--help")...)
		if strings.Contains(out, "idempotency-key") {
			t.Errorf("atmee %s offers --idempotency-key", strings.Join(args, " "))
		}
	}
	if out, _, _ := run(t, "", "avatars", "sponsor", "grant", "--help"); !strings.Contains(out, "idempotency-key") {
		t.Error("sponsor grant keeps --idempotency-key")
	}
}

// update --facts / --facts-file sends knowledge.facts.
func TestUpdateFacts(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	file := filepath.Join(t.TempDir(), "facts.txt")
	_ = os.WriteFile(file, []byte("Favourite tree: oak."), 0o600)
	for args, want := range map[string]string{"--facts=Basil.": "Basil.", "--facts-file=" + file: "Favourite tree: oak."} {
		if _, errOut, code := run(t, "", "avatars", "update", id, args, "--json"); code != 0 {
			t.Fatalf("%s: %d %s", args, code, errOut)
		}
		if k, _ := got["knowledge"].(map[string]any); k["facts"] != want || len(got) != 1 {
			t.Errorf("%s: body %v", args, got)
		}
	}
}

// A create without --image or --voice is a draft: no usage error, no
// candidate options sent, and the next commands are named.
func TestCreateDraft(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	aid := "11111111-2222-4333-8444-555555555555"
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"avatarId":"` + aid + `","kind":"conversational","status":"draft","missingInputs":["image","voice"],"statusUrl":"x","engine":"v2"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	out, errOut, code := run(t, "", "avatars", "create", "--name", "Ada")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if got["seedSelection"] != nil || got["seedPreviews"] != nil || got["engine"] != "v2" || got["assets"] != nil {
		t.Errorf("body %v", got)
	}
	if !strings.Contains(out, "draft") || !strings.Contains(out, "replace-appearance "+aid) || !strings.Contains(out, "replace-voice "+aid) {
		t.Errorf("output: %s", out)
	}
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "Ada", "--candidates", "2"); code == 0 || !strings.Contains(errOut, "need both --image and --voice") {
		t.Errorf("--candidates on a draft: %d %s", code, errOut)
	}
}

func TestGetShowsSettingsAndKnowledge(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/avatars/" + id:
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Mira","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private",` +
				`"createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","stages":{},"language":"ja",` +
				`"persona":{"whoPrompt":"A cheerful gardener.","whatPrompt":"Plan a herb bed.","preferredAlias":"Mimi","guardrails":["No advice."]},` +
				`"profile":{"title":"Mira's garden","subtitle":"Herbs, explained","tags":["garden","herbs"],"socialLinks":[{"type":"youtube","url":"https://youtube.com/@mira"}],` +
				`"statementType":"fictional_verification","verified":true,"monetizationEnabled":true,"thumbnailSource":"custom"},` +
				`"knowledge":{"facts":"Basil.","factsChars":6,"files":[{"id":"f1","name":"guide.pdf"}],"questionsAnswered":1,` +
				`"questionnaire":{"Favourite colour?":"Green"},"integrationIds":["int-1"]}}`))
		case "/v1/avatars/" + id + "/knowledge":
			_, _ = w.Write([]byte(`{"facts":"Basil.","questionnaire":{"Favourite colour?":"Green"},"integrationIds":["int-1"],` +
				`"files":[{"id":"f1","name":"guide.pdf","description":"The guide","url":"https://x/guide.pdf"},{"id":"f2","name":"old.pdf"}],"expiresAt":"2026-10-16T01:00:00Z"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	out, errOut, code := run(t, "", "avatars", "get", id)
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Language: ja", "Alias: Mimi", "Who are you: A cheerful gardener.", "What are we doing: Plan a herb bed.",
		"Title: Mira's garden", "Subtitle: Herbs, explained", "Tags: garden, herbs", "Social: youtube https://youtube.com/@mira",
		"Statement: fictional, verified · monetized: yes · thumbnail: custom", "Guardrails: 1 rule", "Facts: Basil.",
		"Knowledge: 6 characters of facts, 1 file, 1 question answered, 1 integration"} {
		if !strings.Contains(out, want) {
			t.Errorf("get: no %q in\n%s", want, out)
		}
	}
	out, errOut, code = run(t, "", "avatars", "get-knowledge", id)
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Basil.", "Favourite colour?", "Green", "Integrations (news and social feeds): int-1", "guide.pdf — The guide", "https://x/guide.pdf", "old.pdf\n    (no download link"} {
		if !strings.Contains(out, want) {
			t.Errorf("get-knowledge: no %q in\n%s", want, out)
		}
	}
}

func TestFailedDownloadKeepsTheExistingFile(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/avatars/" + id + "/appearance":
			_, _ = w.Write([]byte(`{"url":"` + srv.URL + `/file","expiresAt":"2026-10-16T01:00:00Z"}`))
		case "/file":
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("partial")) // the connection ends short
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	out := filepath.Join(t.TempDir(), "portrait.png")
	if err := os.WriteFile(out, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run(t, "", "avatars", "get-appearance", id, "--out", out); code == 0 {
		t.Fatal("a short download succeeded")
	}
	if b, _ := os.ReadFile(out); string(b) != "keep me" {
		t.Fatalf("the existing file was overwritten: %q", b)
	}
	if entries, _ := os.ReadDir(filepath.Dir(out)); len(entries) != 1 {
		t.Fatalf("a temporary file was left behind: %v", entries)
	}
}

func TestRuntimeSeedFlag(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account" { // the create's v2 access lookup
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet { // the v2 access check
			_, _ = w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"avatarId":"` + id + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + id + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private",` +
			`"engine":"v2","runtimeSeed":7,"createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","stages":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "M", "--runtime-seed", "7", "--json"); code != 0 {
		t.Fatalf("create: %d %s", code, errOut)
	}
	out, errOut, code := run(t, "", "avatars", "update", id, "--runtime-seed", "0")
	if code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	if len(got) != 2 || got[0]["runtimeSeed"] != float64(7) || got[1]["runtimeSeed"] != float64(0) {
		t.Fatalf("bodies: %v", got)
	}
	if !strings.Contains(out, "Runtime seed: 7") {
		t.Errorf("update output has no runtime seed:\n%s", out)
	}
	if _, _, code := run(t, "", "avatars", "update", id, "--runtime-seed", "-1"); code == 0 || len(got) != 2 {
		t.Fatalf("a negative seed was sent: %d %v", code, got)
	}
}

// --config's runtime_seed reaches the create body, and a seed out of range
// is refused before any request.
func TestRuntimeSeedFromConfig(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account" { // the create's v2 access lookup
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"avatarId":"` + id + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + id + `"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if _, errOut, code := run(t, `{"runtime_seed": 11}`, "avatars", "create", "--name", "M", "--config", "-", "--json"); code != 0 {
		t.Fatalf("create: %d %s", code, errOut)
	}
	if len(got) != 1 || got[0]["runtimeSeed"] != float64(11) {
		t.Fatalf("bodies: %v", got)
	}
	if _, _, code := run(t, "", "avatars", "create", "--name", "M", "--runtime-seed", "2147483648"); code == 0 || len(got) != 1 {
		t.Fatalf("an out-of-range seed was sent: %d %v", code, got)
	}
}

// --resolution reaches the create and update bodies as the API's
// {width, height}; a size the renderer cannot take, or one on a v1 create,
// is refused before any request.
func TestResolutionFlag(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { // the v2 access check
			_, _ = w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"avatarId":"` + id + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + id + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private",` +
			`"engine":"v2","resolution":{"width":672,"height":384},"createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","stages":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "M", "--resolution", "384x672", "--json"); code != 0 {
		t.Fatalf("create: %d %s", code, errOut)
	}
	out, errOut, code := run(t, "", "avatars", "update", id, "--resolution", "672X384")
	if code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	want := []map[string]any{{"width": float64(384), "height": float64(672)}, {"width": float64(672), "height": float64(384)}}
	if len(got) != 2 || !reflect.DeepEqual(got[0]["resolution"], want[0]) || !reflect.DeepEqual(got[1]["resolution"], want[1]) {
		t.Fatalf("bodies: %v", got)
	}
	if !strings.Contains(out, "Resolution: 672x384") {
		t.Errorf("update output has no resolution:\n%s", out)
	}
	for _, bad := range [][]string{
		{"avatars", "update", id, "--resolution", "1024x1024"}, // over 512x512 pixels
		{"avatars", "update", id, "--resolution", "500x500"},   // not a multiple of 32
		{"avatars", "update", id, "--resolution", "512"},
		{"avatars", "create", "--name", "M", "--engine", "v1", "--resolution", "512x512"},
	} {
		if _, errOut, code := run(t, "", bad...); code == 0 || len(got) != 2 || errOut == "" {
			t.Fatalf("%v was sent: %d %v", bad, code, got)
		}
	}
}

// --config's resolution takes "WxH" or the API's {width, height}.
func TestResolutionFromConfig(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { // the v2 access check
			_, _ = w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"avatarId":"` + id + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + id + `"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	for _, cfg := range []string{`{"resolution": "384x672"}`, `{"resolution": {"width": 384, "height": 672}}`} {
		if _, errOut, code := run(t, cfg, "avatars", "create", "--name", "M", "--config", "-", "--json"); code != 0 {
			t.Fatalf("%s: %d %s", cfg, code, errOut)
		}
	}
	want := map[string]any{"width": float64(384), "height": float64(672)}
	if len(got) != 2 || !reflect.DeepEqual(got[0]["resolution"], want) || !reflect.DeepEqual(got[1]["resolution"], want) {
		t.Fatalf("bodies: %v", got)
	}
	for _, cfg := range []string{`{"resolution": "big"}`, `{"resolution": {"width": 384}}`, `{"resolution": {"width": 2048, "height": 128}}`} {
		if _, _, code := run(t, cfg, "avatars", "create", "--name", "M", "--config", "-"); code == 0 || len(got) != 2 {
			t.Fatalf("%s was sent: %d %v", cfg, code, got)
		}
	}
}

func TestSpokenNameAndKnownAliasFlags(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account" { // the create's v2 access lookup
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet { // the v2 access check
			_, _ = w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"avatarId":"` + id + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + id + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","stages":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "M", "--spoken-name", "Mee-ra", "--known-alias", "Mimi", "--known-alias", "The Gardener", "--json"); code != 0 {
		t.Fatalf("create: %d %s", code, errOut)
	}
	persona, _ := got[0]["persona"].(map[string]any)
	if persona["spokenName"] != "Mee-ra" || !reflect.DeepEqual(persona["knownAliases"], []any{"Mimi", "The Gardener"}) {
		t.Fatalf("create persona: %v", got[0])
	}
	if _, errOut, code := run(t, "", "avatars", "update", id, "--spoken-name", "Meera", "--known-alias", "Mi", "--json"); code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	if got[1]["spokenName"] != "Meera" || !reflect.DeepEqual(got[1]["knownAliases"], []any{"Mi"}) {
		t.Fatalf("update body: %v", got[1])
	}
	if _, errOut, code := run(t, "", "avatars", "update", id, "--clear", "spoken-name", "--clear", "known-aliases", "--json"); code != 0 {
		t.Fatalf("clear: %d %s", code, errOut)
	}
	if got[2]["spokenName"] != "" || !reflect.DeepEqual(got[2]["knownAliases"], []any{}) {
		t.Fatalf("clear body: %v", got[2])
	}
	if _, _, code := run(t, "", "avatars", "update", id, "--known-alias", "Doe, Jane"); code == 0 || len(got) != 3 {
		t.Fatalf("an alias with a comma was sent: %d %v", code, got)
	}
}

func TestHabitFlag(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	id := "11111111-2222-4333-8444-555555555555"
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.Method {
		case http.MethodPost:
			got = append(got, body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"avatarId":"` + id + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + id + `"}`))
			return
		case http.MethodPatch:
			got = append(got, body)
		}
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private",` +
			`"createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","stages":{},` +
			`"persona":{"conversationalHabits":{"CONVERSATIONAL_HABIT_WORDINESS":"CONVERSATIONAL_HABIT_SCALE_MEDIUM","CONVERSATIONAL_HABIT_FOCUS":"CONVERSATIONAL_HABIT_SCALE_LOW"}}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "M", "--habit", "engagement=high", "--json"); code != 0 {
		t.Fatalf("create: %d %s", code, errOut)
	}
	persona, _ := got[0]["persona"].(map[string]any)
	if !reflect.DeepEqual(persona["conversationalHabits"], map[string]any{"CONVERSATIONAL_HABIT_ENGAGEMENT": "CONVERSATIONAL_HABIT_SCALE_HIGH"}) {
		t.Fatalf("create habits: %v", got[0])
	}
	// Update merges into the avatar's current sliders.
	if _, errOut, code := run(t, "", "avatars", "update", id, "--habit", " Wordiness = High ", "--json"); code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	want := map[string]any{"CONVERSATIONAL_HABIT_WORDINESS": "CONVERSATIONAL_HABIT_SCALE_HIGH", "CONVERSATIONAL_HABIT_FOCUS": "CONVERSATIONAL_HABIT_SCALE_LOW"}
	if !reflect.DeepEqual(got[1]["conversationalHabits"], want) {
		t.Fatalf("update habits: %v", got[1])
	}
	if _, errOut, code := run(t, "", "avatars", "update", id, "--clear", "habits", "--json"); code != 0 || !reflect.DeepEqual(got[2]["conversationalHabits"], map[string]any{}) {
		t.Fatalf("clear: %d %s %v", code, errOut, got)
	}
	for _, bad := range []string{"tone=high", "engagement=extreme", "engagement"} {
		if _, _, code := run(t, "", "avatars", "update", id, "--habit", bad); code == 0 {
			t.Errorf("--habit %s was accepted", bad)
		}
	}
	if len(got) != 3 {
		t.Fatalf("a bad --habit was sent: %v", got)
	}
}

// A create that chooses no engine makes v1 for an account not let into v2
// yet, and says so; an account with access gets v2; a chosen engine is
// sent as is.
func TestCreateEngineFollowsV2Access(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	aid := "11111111-2222-4333-8444-555555555555"
	for _, tc := range []struct {
		access string
		args   []string
		want   string
		note   string // a line on stderr, "" for none
		fails  string // the error, "" when the create succeeds
	}{
		{access: "invite_required", want: "", note: "invite-only"}, // v1, the API's default, is not sent
		{access: "unavailable", want: "", note: "not offered on this server"},
		{access: "invite_required", args: []string{"--seed-selection", "manual"}, fails: "drop them to make a v1 avatar"},
		{access: "granted", want: "v2"},
		{access: "", want: "v2"},
		{access: "invite_required", args: []string{"--engine", "v2"}, want: "v2"},
	} {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v1/account" {
				acct := `{"accountId":"a","plan":{},"limits":{},"pricing":{},"captureQuota":{"pending":0,"pendingLimit":1}`
				if tc.access != "" {
					acct += `,"v2Access":"` + tc.access + `"`
				}
				_, _ = w.Write([]byte(acct + "}"))
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"avatarId":"` + aid + `","kind":"conversational","status":"draft","missingInputs":["image","voice"],"statusUrl":"x"}`))
		}))
		t.Setenv("ATMEE_BASE_URL", srv.URL)
		_, errOut, code := run(t, "", append([]string{"avatars", "create", "--name", "Ada"}, tc.args...)...)
		srv.Close()
		if tc.fails != "" {
			if code == 0 || !strings.Contains(errOut, tc.fails) || got != nil {
				t.Errorf("%+v: %d %s (sent %v)", tc, code, errOut, got)
			}
			continue
		}
		if code != 0 {
			t.Fatalf("%+v: %d %s", tc, code, errOut)
		}
		if e, _ := got["engine"].(string); e != tc.want {
			t.Errorf("%+v: engine %v, want %q", tc, got["engine"], tc.want)
		}
		if noted := strings.Contains(errOut, "Making a v1 avatar"); noted != (tc.note != "") || !strings.Contains(errOut, tc.note) {
			t.Errorf("%+v: stderr %q", tc, errOut)
		}
	}
}

func jsonOf(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, s)
	}
	return m
}

// ok runs the CLI and fails the test on a non-zero exit.
func ok(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("atmee %s: exit %d\n%s%s", strings.Join(args, " "), code, out, errOut)
	}
	return out
}
