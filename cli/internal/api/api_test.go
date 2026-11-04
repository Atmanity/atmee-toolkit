package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const aid = "11111111-2222-4333-8444-555555555555"

func newTest(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, APIKey("sk_atmee_test"), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.Sleep = func(time.Duration) {}
	// The warming wait goes through c.Sleep too, so a test's Sleep hears it.
	c.sleepCtx = func(ctx context.Context, d time.Duration) error { c.Sleep(d); return ctx.Err() }
	c.InProgressSleep = 0
	return c
}

func TestNewNeedsAKey(t *testing.T) {
	if _, err := New("http://x", APIKey(" "), time.Second); err == nil || !strings.Contains(err.Error(), "atmee login") {
		t.Fatalf("got %v", err)
	}
}

func TestErrorsCarryCodeRetryAfterAndTrimTheSchemaDump(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/candidates") {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":"rate_limited","message":"10 captures are already queued or running","reason":"pending_cap"}`))
			return
		}
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"code":422,"message":"request body has an error: x\nSchema:\n {huge}"}`))
	})
	_, err := c.AddCandidates(context.Background(), aid, 1)
	var e *Error
	if !errors.As(err, &e) || e.Status != 429 || e.Code != "rate_limited" || e.RetryAfter != 30 || e.Reason != "pending_cap" {
		t.Fatalf("429: %#v", err)
	}
	if !strings.Contains(err.Error(), "(retry in 30s)") {
		t.Fatalf("429 text: %v", err)
	}
	_, err = c.Update(context.Background(), aid, map[string]any{"name": "x"})
	if !errors.As(err, &e) || e.Message != "request body has an error: x" {
		t.Fatalf("422 message: %q", err)
	}
}

func TestIdempotentRetryKeepsOneKey(t *testing.T) {
	var calls atomic.Int32
	keys := map[string]bool{}
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		keys[r.Header.Get("Idempotency-Key")] = true
		if calls.Add(1) == 1 {
			// The first attempt is still running on the server.
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"error":"idempotency_key_in_progress","message":"still running"}`))
			return
		}
		_, _ = w.Write([]byte(`{"transferredMicrocredits":1000,"replayed":true}`))
	})
	if _, err := c.GrantSponsor(context.Background(), aid, 1000, ""); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || len(keys) != 1 {
		t.Fatalf("an in-progress retry must reuse the key: calls %d keys %v", calls.Load(), keys)
	}
}

func TestTransportFailureReportsTheKeyToRetryWith(t *testing.T) {
	c, _ := New("http://127.0.0.1:1", APIKey("k"), time.Second)
	c.Sleep = func(time.Duration) {}
	_, err := c.GrantSponsor(context.Background(), aid, 1000, "my-key")
	if err == nil || !strings.Contains(err.Error(), "Idempotency-Key my-key") {
		t.Fatalf("got %v", err)
	}
}

func TestCapturesAreSentOnceWithoutAKey(t *testing.T) {
	var calls atomic.Int32
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if k := r.Header.Get("Idempotency-Key"); k != "" {
			t.Errorf("%s %s sent Idempotency-Key %q", r.Method, r.URL.Path, k)
		}
		w.WriteHeader(202)
		if strings.HasSuffix(r.URL.Path, "/candidates") {
			_, _ = w.Write([]byte(`{"items":[],"generation":1,"count":0,"seedSelection":"manual","selectionRequired":false,"rebuilding":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"avatarId":"` + aid + `","status":"building"}`))
	})
	if _, err := c.AddCandidates(context.Background(), aid, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Create(context.Background(), CreateInput{Name: "n", Image: "https://x/i.png", Voice: "https://x/v.webm"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls %d", calls.Load())
	}

	// A transport failure is not resent; the error says what to check.
	c, _ = New("http://127.0.0.1:1", APIKey("k"), time.Second)
	_, err := c.AddCandidates(context.Background(), aid, 1)
	if err == nil || !strings.Contains(err.Error(), "may have reached the server") || !strings.Contains(err.Error(), "candidates list "+aid) {
		t.Fatalf("add: %v", err)
	}
	_, _, err = c.Create(context.Background(), CreateInput{Name: "n", Image: "https://x/i.png", Voice: "https://x/v.webm"})
	if err == nil || !strings.Contains(err.Error(), "atmee avatars list") {
		t.Fatalf("create: %v", err)
	}
}

func TestQuotaAndCandidateHelpers(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "10")
		w.Header().Set("X-RateLimit-Remaining", "7")
		_, _ = w.Write([]byte(`{"generation":2,"count":3,"seedSelection":"manual","selectionRequired":false,"rebuilding":true,"items":[
			{"candidateId":"aaaaaaaa-0000-4000-8000-000000000001","generation":1,"seed":1,"status":"pending","serving":false,"createdAt":"2026-10-16T00:00:00Z"},
			{"candidateId":"aaaaaaaa-0000-4000-8000-000000000002","generation":2,"seed":2,"status":"completed","serving":false,"createdAt":"2026-10-16T00:00:00Z","previewUrl":"u"},
			{"candidateId":"aaaaaaaa-0000-4000-8000-000000000003","generation":2,"seed":3,"status":"processing","serving":false,"createdAt":"2026-10-16T00:00:00Z"}]}`))
	})
	p, err := c.Candidates(context.Background(), aid)
	if err != nil {
		t.Fatal(err)
	}
	if p.Quota == nil || *p.Quota != (Quota{Limit: 10, Remaining: 7}) {
		t.Fatalf("quota: %+v", p.Quota)
	}
	if len(Current(p.CandidateList)) != 2 || len(Previewable(p.CandidateList)) != 1 || pending(p.CandidateList) != 1 {
		t.Fatal("only current-generation candidates count")
	}
}

// With `current` sent, it decides: a version's takes can span capture rounds
// (generations), and an older version's serving take is listed too.
func TestCurrentFollowsTheVersion(t *testing.T) {
	yes, no := true, false
	cl := &CandidateList{Generation: 3, Items: []Candidate{
		{Generation: 1, Current: &no, Status: "completed"},
		{Generation: 2, Current: &yes, Status: "completed"},
		{Generation: 3, Current: &yes, Status: "pending"},
	}}
	if got := Current(cl); len(got) != 2 || got[0].Generation != 2 || got[1].Generation != 3 {
		t.Fatalf("current: %+v", got)
	}
}

func TestSettled(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		status    string
		rebuild   *bool
		selection *bool
		want      bool
	}{
		{"building", nil, nil, false},
		{"building", nil, &yes, true},
		{"ready", nil, nil, true},
		{"ready", &yes, nil, false},
		{"ready", &no, nil, true},
		{"failed", &yes, nil, true},
		{"moderation_hold", nil, nil, true},
		{"draft", nil, nil, true}, // waits for its portrait or voice, not for time
	}
	for _, tc := range cases {
		a := &Avatar{Rebuilding: tc.rebuild, SelectionRequired: tc.selection}
		a.Status = Avatar{}.Status
		b, _ := json.Marshal(map[string]any{"status": tc.status})
		_ = json.Unmarshal(b, a)
		if got := Settled(a); got != tc.want {
			t.Errorf("%s rebuilding=%v selection=%v: %v", tc.status, tc.rebuild, tc.selection, got)
		}
	}
}

func TestManifestRules(t *testing.T) {
	// Without the voice (or the portrait) a v2 create is a draft: allowed,
	// but its first take is the API's (one, automatic).
	if _, err := (CreateInput{Name: "x", Image: "i"}).Manifest(); err != nil {
		t.Fatalf("v2 draft without a voice: %v", err)
	}
	for _, in := range []CreateInput{{Name: "x", Image: "i", Candidates: 2}, {Name: "x", Voice: "v", SeedSelection: "manual"}} {
		if _, err := in.Manifest(); err == nil || !strings.Contains(err.Error(), "need both --image and --voice") {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if _, err := (CreateInput{Name: "x", Image: "i", Voice: "v", Candidates: 11}).Manifest(); err == nil {
		t.Fatal("11 candidates")
	}
	if _, err := (CreateInput{Name: "x", Image: "i", Voice: "v", PreviewMessage: strings.Repeat("話", 301)}).Manifest(); err == nil {
		t.Fatal("301-character preview message")
	}
	m, err := (CreateInput{Name: "x", Image: "i", Engine: "v1", PreviewMessage: strings.Repeat("x", 80)}).Manifest()
	if err != nil || m["engine"] != nil {
		t.Fatalf("v1 has no engine field and no prime limit: %v %v", m, err)
	}
	m, _ = (CreateInput{Name: "x", Image: "i", Voice: "v", Candidates: 2, SeedSelection: "manual", Facts: "f",
		Persona: map[string]any{"greeting": "hi"}}).Manifest()
	if m["engine"] != "v2" || m["seedPreviews"].(map[string]int)["count"] != 2 || m["persona"].(map[string]any)["greeting"] != "hi" {
		t.Fatalf("manifest: %v", m)
	}
}

func TestCreateFromLocalFilesSendsADeterministicBundle(t *testing.T) {
	dir := t.TempDir()
	img, voice := filepath.Join(dir, "p.png"), filepath.Join(dir, "v.wav")
	_ = os.WriteFile(img, []byte("png"), 0o600)
	_ = os.WriteFile(voice, []byte("wav"), 0o600)
	var bodies [][]byte
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		if r.Header.Get("X-Mock-Scenario") != "stuck" || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("headers: %v", r.Header)
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"avatarId":"` + aid + `","kind":"conversational","status":"building","statusUrl":"x","engine":"v2"}`))
	})
	c.MockScenario = "stuck" // sent on every request
	in := CreateInput{Name: "M", Image: img, Voice: voice}
	for i := 0; i < 2; i++ {
		if _, _, err := c.Create(context.Background(), in); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond) // a zip stamped with the clock would differ now
	}
	if !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatal("a retry must send the same bytes")
	}
	zipStart := bytes.Index(bodies[0], []byte("PK\x03\x04"))
	zr, err := zip.NewReader(bytes.NewReader(bodies[0][zipStart:]), int64(len(bodies[0])-zipStart))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "avatar.json,image/p.png,voice/v.wav" {
		t.Fatalf("entries: %v", names)
	}
}

func TestCreateRefusesMixedURLsAndFiles(t *testing.T) {
	c := newTest(t, func(http.ResponseWriter, *http.Request) { t.Error("no request expected") })
	_, _, err := c.Create(context.Background(), CreateInput{Name: "x", Image: "https://x/p.png", Voice: "local.wav"})
	if err == nil || !strings.Contains(err.Error(), "or all as local files") {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateChecksTheV2PrimeLimitOnlyForV2(t *testing.T) {
	engine := "v2"
	patched := false
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patched = true
		}
		_, _ = w.Write([]byte(`{"id":"` + aid + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","engine":"` + engine + `"}`))
	})
	long := strings.Repeat("x", 301)
	if _, err := c.Update(context.Background(), aid, map[string]any{"previewMessage": long}); err == nil || patched {
		t.Fatal("v2 refuses a 301-character preview message before patching")
	}
	engine = "v1"
	if _, err := c.Update(context.Background(), aid, map[string]any{"previewMessage": long}); err != nil || !patched {
		t.Fatalf("v1 has no limit: %v", err)
	}
}

func TestUpdateSendsEmptyValuesAsGiven(t *testing.T) {
	var got map[string]any
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"` + aid + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z"}`))
	})
	if _, err := c.Update(context.Background(), aid, map[string]any{"description": "", "guardrails": []string{}}); err != nil {
		t.Fatal(err)
	}
	if gs, ok := got["guardrails"].([]any); got["description"] != "" || !ok || len(gs) != 0 {
		t.Fatalf("an empty value removes it, not null: %v", got)
	}
}

func TestStartSessionChecksTheVisitorID(t *testing.T) {
	c := newTest(t, func(http.ResponseWriter, *http.Request) { t.Error("no request expected") })
	if _, err := c.StartSession(context.Background(), aid, EmbedSessionInput{VisitorID: "short"}); err == nil {
		t.Fatal("a short visitor id is guessable")
	}
}

func TestStartSessionSendsWelcomeOnlyWhenOff(t *testing.T) {
	var bodies []map[string]any
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sessionId":"` + aid + `","serverUrl":"wss://lk.test","roomName":"r","userToken":"t","billingMode":"reserve"}`))
	})
	for _, in := range []EmbedSessionInput{{}, {NoWelcome: true}} {
		if _, err := c.StartSession(context.Background(), aid, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, sent := bodies[0]["welcome"]; sent {
		t.Fatalf("the default sent welcome: %v", bodies[0])
	}
	if bodies[1]["welcome"] != false {
		t.Fatalf("no-welcome: %v", bodies[1])
	}
}

func TestStartSessionSendsOnlyAChosenCheckIn(t *testing.T) {
	var bodies []map[string]any
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sessionId":"` + aid + `","serverUrl":"wss://lk.test","roomName":"r","userToken":"t","billingMode":"reserve"}`))
	})
	off, later := 0.0, 45.5
	for _, in := range []EmbedSessionInput{{}, {CheckInAfterS: &off}, {CheckInAfterS: &later}} {
		if _, err := c.StartSession(context.Background(), aid, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, sent := bodies[0]["checkInAfterS"]; sent {
		t.Fatalf("the default sent a check-in: %v", bodies[0])
	}
	if bodies[1]["checkInAfterS"] != 0.0 || bodies[2]["checkInAfterS"] != 45.5 {
		t.Fatalf("check-ins: %v, %v", bodies[1], bodies[2])
	}
	for _, bad := range []float64{-1, 3600.5, math.NaN()} {
		if _, err := c.StartSession(context.Background(), aid, EmbedSessionInput{CheckInAfterS: &bad}); err == nil || !strings.Contains(err.Error(), "check-in") {
			t.Fatalf("%v: err = %v", bad, err)
		}
	}
	if len(bodies) != 3 {
		t.Fatalf("%d requests, want the 3 in range", len(bodies))
	}
}

func TestIDsAreValidatedLocally(t *testing.T) {
	c := newTest(t, func(http.ResponseWriter, *http.Request) { t.Error("no request expected") })
	if _, err := c.Get(context.Background(), "nope"); err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("got %v", err)
	}
}

func TestSponsorGrantsAreAlwaysKeyed(t *testing.T) {
	var keys []string
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		_, _ = w.Write([]byte(`{"transferredMicrocredits":1000,"replayed":false}`))
	})
	if _, err := c.GrantSponsor(context.Background(), aid, 1000, ""); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] == "" {
		t.Fatalf("a grant without a key must get one: %q", keys)
	}
}

func TestVisitorIDCountsCharacters(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"sessionId":"` + aid + `","roomName":"r","serverUrl":"wss://x","userToken":"t","billingMode":"reserve","actualDurationSeconds":1}`))
	})
	if _, err := c.StartSession(context.Background(), aid, EmbedSessionInput{VisitorID: strings.Repeat("訪", 100)}); err != nil {
		t.Fatalf("100 characters fit: %v", err)
	}
	if _, err := c.StartSession(context.Background(), aid, EmbedSessionInput{VisitorID: strings.Repeat("訪", 10)}); err == nil {
		t.Fatal("10 characters (30 bytes) are too short")
	}
}

func TestInProgressTimeoutHandsBackTheKey(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":"idempotency_key_in_progress","message":"still running"}`))
	})
	_, err := c.GrantSponsor(context.Background(), aid, 1000, "my-key")
	if err == nil || !strings.Contains(err.Error(), "Idempotency-Key my-key") {
		t.Fatalf("got %v", err)
	}
}

func TestWaitReportsOnlyChangedStates(t *testing.T) {
	n := 0
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		n++
		status := "building"
		if n >= 3 {
			status = "ready"
		}
		// Rebuilding is a fresh *bool on every decode: identical polls must
		// still count as unchanged.
		_, _ = w.Write([]byte(`{"id":"` + aid + `","name":"M","kind":"conversational","status":"` + status + `","readyToChat":` + strconv.FormatBool(n >= 3) + `,"rebuilding":false,"visibility":"private","createdAt":"2026-10-17T00:00:00Z"}`))
	})
	c.Sleep = func(time.Duration) {}
	var seen []string
	if _, err := c.Wait(context.Background(), aid, time.Minute, 0, func(a *Avatar) { seen = append(seen, string(a.Status)) }); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "building,ready" {
		t.Fatalf("updates: %v", seen)
	}
}

func TestPlainHTTPAssetsAreRefused(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) { t.Fatal("must not reach the API") })
	_, _, err := c.Create(context.Background(), CreateInput{Name: "M", Image: "http://x/p.png", Voice: "https://x/v.wav"})
	if err == nil || !strings.Contains(err.Error(), "https://") {
		t.Fatalf("got %v", err)
	}
}

func TestRetryAfterOn503(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":"avatar_warming","message":"the pool is starting"}`))
	})
	_, err := c.Get(context.Background(), aid)
	if err == nil || !strings.Contains(err.Error(), "retry in 42s") {
		t.Fatalf("got %v", err)
	}
}

func TestSponsorOnCreateManifest(t *testing.T) {
	micro, capPer := 1000, 200
	m, err := CreateInput{Name: "M", Image: "https://x/p.png", Voice: "https://x/v.wav", SponsorMicrocredits: &micro, SponsorMaxPerSession: &capPer}.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if sp, _ := m["sponsor"].(map[string]int); sp["microcredits"] != 1000 || sp["maxPerSession"] != 200 {
		t.Fatalf("sponsor: %v", m["sponsor"])
	}
	zero := 0
	if _, err := (CreateInput{Name: "M", Image: "https://x/p.png", Voice: "https://x/v.wav", SponsorMicrocredits: &zero}).Manifest(); err == nil {
		t.Fatal("a zero grant is refused before the request")
	}
}

func TestWaitForTheRequestedCandidates(t *testing.T) {
	const cid = "11111111-2222-4333-8444-555555555555"
	old := `{"candidateId":"aaaaaaaa-2222-4333-8444-555555555555","generation":1,"seed":1,"status":"completed","createdAt":"2026-10-17T00:00:00Z"}`
	calls := 0
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		items := old // the first read does not list the new capture yet
		switch {
		case calls == 2:
			items += `,{"candidateId":"` + cid + `","generation":1,"seed":2,"status":"pending","createdAt":"2026-10-17T00:00:01Z"}`
		case calls >= 3:
			items += `,{"candidateId":"` + cid + `","generation":1,"seed":2,"status":"completed","createdAt":"2026-10-17T00:00:01Z"}`
		}
		_, _ = w.Write([]byte(`{"items":[` + items + `],"generation":1,"seedSelection":"manual","selectionRequired":true,"rebuilding":false}`))
	})
	c.Sleep = func(time.Duration) {}
	if _, err := c.WaitForCandidates(context.Background(), aid, time.Minute, 0, cid); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("returned after %d reads; the requested capture settles on the third", calls)
	}
}

func TestNewIDsAreTheCapturesARequestAdded(t *testing.T) {
	const older, queued, added = "aaaaaaaa-2222-4333-8444-555555555555", "bbbbbbbb-2222-4333-8444-555555555555", "cccccccc-2222-4333-8444-555555555555"
	item := func(id string, gen int, status string) string {
		return `{"candidateId":"` + id + `","generation":` + strconv.Itoa(gen) + `,"seed":1,"status":"` + status + `","createdAt":"2026-10-17T00:00:00Z"}`
	}
	list := func(items ...string) *CandidateList {
		var cl CandidateList
		if err := json.Unmarshal([]byte(`{"generation":2,"seedSelection":"manual","selectionRequired":false,"rebuilding":true,"items":[`+strings.Join(items, ",")+`]}`), &cl); err != nil {
			t.Fatal(err)
		}
		return &cl
	}
	// An older generation's capture and one still queued from an earlier
	// request are listed before and after; only the new one is this request's.
	before := list(item(older, 1, "pending"), item(queued, 2, "pending"))
	after := list(item(older, 1, "pending"), item(queued, 2, "pending"), item(added, 2, "pending"))
	ids := NewIDs(before, after)
	if strings.Join(ids, ",") != added {
		t.Fatalf("new ids: %v", ids)
	}
	// Nothing new: no ids, so a wait covers every current capture.
	if ids := NewIDs(after, after); len(ids) != 0 {
		t.Fatalf("nothing new: %v", ids)
	}

	// The wait ends once the new capture settles, while the queued one is
	// still capturing.
	calls := 0
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"generation":2,"seedSelection":"manual","selectionRequired":true,"rebuilding":false,"items":[` +
			item(queued, 2, "processing") + `,` + item(added, 2, "completed") + `]}`))
	})
	if _, err := c.WaitForCandidates(context.Background(), aid, 200*time.Millisecond, 0, ids...); err != nil || calls != 1 {
		t.Fatalf("waited on an older capture: %v after %d reads", err, calls)
	}
}

func TestWaitServingComparesTheIDs(t *testing.T) {
	const pick, other = "aaaaaaaa-0000-4000-8000-000000000001", "aaaaaaaa-0000-4000-8000-000000000002"
	body := func(status, serving, selected string, rebuilding bool) string {
		b, _ := json.Marshal(map[string]any{"id": aid, "name": "x", "status": status, "rebuilding": rebuilding,
			"servingCandidateId": serving, "selectedCandidateId": selected})
		return string(b)
	}
	for _, tc := range []struct {
		name       string
		polls      []string
		wantErr    string
		wantFailed bool
		wantN      int
	}{
		// ready and not rebuilding, but still serving the old take: keep waiting.
		{"waits past a ready avatar serving the old take", []string{
			body("ready", other, pick, false), body("ready", pick, pick, false)}, "", false, 2},
		{"a newer pick ends the wait", []string{body("ready", other, other, false)}, "was picked meanwhile", false, 1},
		// The pick still serves while a newer one builds to replace it.
		{"a newer pick ends it while the pick still serves", []string{
			body("building", pick, other, true)}, "was picked meanwhile", false, 1},
		{"a failed build ends it", []string{body("failed", other, pick, false)}, "", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var n atomic.Int32
			c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
				i := int(n.Add(1)) - 1
				if i >= len(tc.polls) {
					i = len(tc.polls) - 1
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.polls[i]))
			})
			_, err := c.WaitServing(context.Background(), aid, pick, time.Minute, 0)
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if tc.wantFailed {
				var bf *BuildFailedError
				if !errors.As(err, &bf) {
					t.Fatalf("err = %v, want BuildFailedError", err)
				}
			} else if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if int(n.Load()) != tc.wantN {
				t.Fatalf("polls = %d, want %d", n.Load(), tc.wantN)
			}
		})
	}
}

func TestSelectAndWaitAcceptAnyUUIDSpelling(t *testing.T) {
	const pick = "7e8e5254-0000-4000-8000-000000000001"
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["candidateId"] != pick {
				t.Errorf("sent %q, want the canonical id", b["candidateId"])
			}
		}
		// The API answers with lowercase ids, whatever the request spelled.
		b, _ := json.Marshal(map[string]any{"id": aid, "name": "x", "status": "ready",
			"servingCandidateId": pick, "selectedCandidateId": pick})
		_, _ = w.Write(b)
	})
	for _, spelling := range []string{strings.ToUpper(pick), "{" + pick + "}"} {
		if _, err := c.SelectCandidate(context.Background(), aid, spelling); err != nil {
			t.Fatalf("select %s: %v", spelling, err)
		}
		if _, err := c.WaitServing(context.Background(), aid, spelling, time.Minute, 0); err != nil {
			t.Fatalf("wait %s: %v", spelling, err)
		}
	}
	if _, err := c.SelectCandidate(context.Background(), aid, "nope"); err == nil || !strings.Contains(err.Error(), "not a candidate id") {
		t.Fatalf("a bad id fails before a request: %v", err)
	}
}

// warmingServer answers a session start with 503 code warm times (forever
// when warm < 0), then with ok. An avatar read answers with engine: "v1",
// "v2", "" (no engine field) or "forbidden" (403, a key without avatars:read).
func warmingServer(t *testing.T, warm int, retryAfter, code, ok, engine string) (c *Client, starts, reads *atomic.Int32) {
	t.Helper()
	starts, reads = &atomic.Int32{}, &atomic.Int32{}
	c = newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			reads.Add(1)
			if engine == "forbidden" {
				w.WriteHeader(403)
				_, _ = w.Write([]byte(`{"error":"insufficient_scope","message":"needs avatars:read"}`))
				return
			}
			a := map[string]any{"id": aid, "name": "x", "status": "ready"}
			if engine != "" {
				a["engine"] = engine
			}
			b, _ := json.Marshal(a)
			_, _ = w.Write(b)
			return
		}
		if n := int(starts.Add(1)); warm < 0 || n <= warm {
			if retryAfter != "" {
				w.Header().Set("Retry-After", retryAfter)
			}
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":"` + code + `","message":"not yet"}`))
			return
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(ok))
	})
	return c, starts, reads
}

func startAvatarOnly(c *Client) error {
	_, err := c.StartAvatarSession(context.Background(), aid, AvatarSessionInput{LiveKitURL: "wss://lk", LiveKitToken: "t"})
	return err
}

func TestV2AvatarOnlyStartsRetryWhileTheAvatarWarms(t *testing.T) {
	c, starts, reads := warmingServer(t, 2, "1", "avatar_warming", `{"sessionId":"`+aid+`","status":"starting"}`, "v2")
	var slept, heard []time.Duration
	c.Sleep = func(d time.Duration) { slept = append(slept, d) }
	c.OnWarming = func(d time.Duration) { heard = append(heard, d) }
	if err := startAvatarOnly(c); err != nil {
		t.Fatal(err)
	}
	// Retry-After 1 is raised to the 5 s floor; the engine is read once.
	if starts.Load() != 3 || reads.Load() != 1 || len(slept) != 2 || slept[0] != 5*time.Second || len(heard) != 2 {
		t.Fatalf("starts %d, reads %d, slept %v, heard %v", starts.Load(), reads.Load(), slept, heard)
	}
}

func TestWarmingIsReportedAtOnceUnlessTheAvatarIsV2(t *testing.T) {
	for _, engine := range []string{"v1", "", "forbidden"} {
		t.Run("engine "+engine, func(t *testing.T) {
			c, starts, reads := warmingServer(t, -1, "30", "avatar_warming", "", engine)
			c.Sleep = func(time.Duration) { t.Fatal("only a v2 avatar is retried") }
			c.OnWarming = func(time.Duration) { t.Fatal("no warming note for a start that is not retried") }
			err := startAvatarOnly(c)
			var e *Error
			if !errors.As(err, &e) || e.Code != "avatar_warming" || e.RetryAfter != 30 {
				t.Fatalf("want the first avatar_warming answer, got %v", err)
			}
			if starts.Load() != 1 || reads.Load() != 1 {
				t.Fatalf("starts %d, reads %d", starts.Load(), reads.Load())
			}
		})
	}
}

func TestFullSessionStartsReportWarmingAtOnce(t *testing.T) {
	c, starts, reads := warmingServer(t, -1, "30", "avatar_warming", "", "v2")
	c.Sleep = func(time.Duration) { t.Fatal("a full session start is not retried") }
	_, err := c.StartSession(context.Background(), aid, EmbedSessionInput{})
	var e *Error
	if !errors.As(err, &e) || e.Code != "avatar_warming" || starts.Load() != 1 || reads.Load() != 0 {
		t.Fatalf("starts %d, reads %d, err %v", starts.Load(), reads.Load(), err)
	}
}

func TestWarmingRetriesStopAtTheWarmingTimeout(t *testing.T) {
	c, starts, reads := warmingServer(t, -1, "", "avatar_warming", "", "v2")
	clock := time.Now()
	c.now = func() time.Time { return clock }
	var slept time.Duration
	c.Sleep = func(d time.Duration) {
		if slept += d; slept > JoinWaitTimeout {
			t.Fatalf("still retrying after %s", slept)
		}
		clock = clock.Add(d)
	}
	err := startAvatarOnly(c)
	var e *Error
	if !errors.As(err, &e) || e.Code != "avatar_warming" {
		t.Fatalf("want the last avatar_warming answer, got %v", err)
	}
	// No Retry-After: 30 s apart, never past the 10 minute bound.
	if starts.Load() != 21 || reads.Load() != 1 || slept != JoinWaitTimeout {
		t.Fatalf("starts %d, reads %d, slept %s", starts.Load(), reads.Load(), slept)
	}
}

// The join wait starts again for each attempt: a start that gets through
// after the avatar warmed up still has all of JoinTimeout to join.
func TestEachWarmingRetryGetsTheWholeJoinWait(t *testing.T) {
	var starts atomic.Int32
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"` + aid + `","name":"x","status":"ready","engine":"v2"}`))
			return
		}
		if starts.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":"avatar_warming","message":"not yet"}`))
			return
		}
		time.Sleep(300 * time.Millisecond) // the join takes most of JoinTimeout
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"sessionId":"` + aid + `","status":"joined"}`))
	})
	c.JoinTimeout = 500 * time.Millisecond
	// The 5 s warming wait fits the warming budget; it takes 300 ms of real
	// time, which a join wait shared with the retries would lose.
	c.WarmingTimeout = time.Minute
	c.Sleep = func(time.Duration) { time.Sleep(300 * time.Millisecond) }
	_, err := c.StartAvatarSession(context.Background(), aid, AvatarSessionInput{LiveKitURL: "wss://lk", LiveKitToken: "t", WaitJoined: true})
	if err != nil || starts.Load() != 2 {
		t.Fatalf("starts %d, err %v", starts.Load(), err)
	}
}

// Cancelling a start while it waits out avatar_warming ends the wait at once
// instead of after Retry-After.
func TestCancellingDuringAWarmingWaitReturnsPromptly(t *testing.T) {
	c, starts, _ := warmingServer(t, -1, "30", "avatar_warming", "", "v2")
	c.sleepCtx = sleepCtx // the real wait, not the test's no-op
	waiting := make(chan struct{})
	c.OnWarming = func(time.Duration) { close(waiting) }
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-waiting; cancel() }()
	began := time.Now()
	_, err := c.StartAvatarSession(ctx, aid, AvatarSessionInput{LiveKitURL: "wss://lk", LiveKitToken: "t"})
	if took := time.Since(began); !errors.Is(err, context.Canceled) || took > 5*time.Second || starts.Load() != 1 {
		t.Fatalf("want context.Canceled promptly after one start, got %v after %s (starts %d)", err, took, starts.Load())
	}
}

func TestSleepCtxWaitsOutTheDurationUnlessCancelled(t *testing.T) {
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("a finished wait returns nil, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	began := time.Now()
	if err := sleepCtx(ctx, time.Minute); !errors.Is(err, context.Canceled) || time.Since(began) > 5*time.Second {
		t.Fatalf("a cancelled wait returns ctx's error at once, got %v after %s", err, time.Since(began))
	}
}

func TestNoCapacityIsNotRetried(t *testing.T) {
	c, starts, reads := warmingServer(t, -1, "5", "no_capacity", "", "v2")
	c.Sleep = func(time.Duration) { t.Fatal("no_capacity must not be retried") }
	if err := startAvatarOnly(c); err == nil || starts.Load() != 1 || reads.Load() != 0 {
		t.Fatalf("starts %d, reads %d, err %v", starts.Load(), reads.Load(), err)
	}
}

func TestOnlyTheJoinWaitOutlivesTheRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond) // answers past the request timeout
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"sessionId":"` + aid + `","status":"starting"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, APIKey("sk_atmee_test"), 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := startAvatarOnly(c); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("without --wait-joined the start keeps the request timeout: %v", err)
	}
	in := AvatarSessionInput{LiveKitURL: "wss://lk", LiveKitToken: "t", WaitJoined: true}
	if _, err := c.StartAvatarSession(context.Background(), aid, in); err != nil {
		t.Fatalf("--wait-joined gets the join timeout: %v", err)
	}
}

// A create without files sends the manifest alone; one with only a local
// voice zips no image folder.
func TestCreateDraftBodies(t *testing.T) {
	var ct string
	var body []byte
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"avatarId":"` + aid + `","kind":"conversational","status":"draft","missingInputs":["image","voice"],"statusUrl":"x","engine":"v2"}`))
	})
	res, _, err := c.Create(context.Background(), CreateInput{Name: "D", Engine: "v2", Facts: "Basil."})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if ct != "application/json" || json.Unmarshal(body, &m) != nil || m["assets"] != nil ||
		m["knowledge"].(map[string]any)["facts"] != "Basil." {
		t.Fatalf("%s %s", ct, body)
	}
	if res.Status != "draft" || res.MissingInputs == nil || len(*res.MissingInputs) != 2 {
		t.Fatalf("result %+v", res)
	}

	voice := filepath.Join(t.TempDir(), "v.wav")
	_ = os.WriteFile(voice, []byte("wav"), 0o600)
	if _, _, err := c.Create(context.Background(), CreateInput{Name: "D", Engine: "v2", Voice: voice}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ct, "multipart/form-data") || !bytes.Contains(body, []byte("voice/v.wav")) || bytes.Contains(body, []byte("image/")) {
		t.Fatalf("zip of a voice-only draft: %s", ct)
	}
}

// Only the shape of a clip size is checked locally: which sizes a server
// serves (an older one: 512x512, 384x672, ...) is the server's to say.
func TestParseClipResolution(t *testing.T) {
	for in, ok := range map[string]bool{
		"480x480": true, "480 x 640": true, "468X832": true, "1080x1080": true, "512x512": true, "384x672": true, "1920x1080": true,
		"481x480": false, "0x480": false, "-480x480": false, "1920": false, "x480": false, "": false,
	} {
		r, err := ParseClipResolution(in)
		if (err == nil) != ok {
			t.Errorf("ParseClipResolution(%q) = %v, %v; want ok=%v", in, r, err, ok)
		}
	}
}

func TestManifestResolution(t *testing.T) {
	m, err := (CreateInput{Name: "x", Resolution: &Resolution{Width: 384, Height: 640}}).Manifest()
	if err != nil || m["aspectRatio"] != nil || m["resolution"] == nil {
		t.Fatalf("%v %v", m, err)
	}
	if _, err := (CreateInput{Name: "x", Engine: "v1", Resolution: &Resolution{Width: 384, Height: 640}}).Manifest(); err == nil {
		t.Error("v1 with a resolution: no error")
	}
}

func TestParseResolution(t *testing.T) {
	for in, ok := range map[string]bool{
		"512x512": true, "384x640": true, "640 x 384": true, "256x1024": true, "1024X256": true, "256x256": true,
		"640x640": true, "864x480": true, "544x512": true, // V2 HD's rule; each engine's is checked later
		"896x480": false, "1024x1024": false, "224x512": false, "500x500": false, "512": false, "x512": false, "512x512x1": false, "": false,
	} {
		r, err := ParseResolution(in)
		if (err == nil) != ok {
			t.Errorf("ParseResolution(%q) = %v, %v; want ok=%v", in, r, err, ok)
		}
	}
}

// Each engine has its own frame rule: v2 at most 512x512 pixels, V2 HD at
// most 864x480.
func TestCheckResolutionFor(t *testing.T) {
	for _, c := range []struct {
		engine string
		w, h   int
		ok     bool
	}{
		{"v2", 512, 512, true}, {"v2", 384, 640, true}, {"v2", 544, 512, false}, {"v2", 640, 640, false},
		{"v2pro", 640, 640, true}, {"v2pro", 864, 480, true}, {"v2pro", 480, 864, true}, {"v2pro", 736, 544, true},
		{"v2pro", 896, 480, false}, {"v2pro", 1024, 1024, false},
	} {
		err := CheckResolutionFor(c.engine, Resolution{Width: c.w, Height: c.h})
		if (err == nil) != c.ok {
			t.Errorf("%s %dx%d: %v, want ok=%v", c.engine, c.w, c.h, err, c.ok)
		}
	}
	var pro *V2ProSizeError
	if err := CheckResolutionFor("v2", Resolution{Width: 640, Height: 640}); !errors.As(err, &pro) || !strings.Contains(err.Error(), "for V2 HD avatars") {
		t.Errorf("a V2 HD size on v2 does not say it is one: %v", err)
	}
	if err := CheckResolutionFor("v2", Resolution{Width: 1024, Height: 1024}); errors.As(err, &pro) {
		t.Errorf("a size too large for V2 HD too is called a V2 HD size: %v", err)
	}
}

// The sizes a server lists for an engine are checked before sending; a
// server that lists none decides itself.
func TestCheckOffered(t *testing.T) {
	v2 := []Resolution{{Width: 512, Height: 512}}
	pro := []Resolution{{Width: 640, Height: 640}, {Width: 864, Height: 480}}
	acct := &Account{}
	acct.Limits.V2Resolutions, acct.Limits.V2ProResolutions = &v2, &pro
	if err := CheckOffered(acct, "v2pro", Resolution{Width: 864, Height: 480}); err != nil {
		t.Error(err)
	}
	err := CheckOffered(acct, "v2pro", Resolution{Width: 512, Height: 512})
	if err == nil || !strings.Contains(err.Error(), "not a V2 HD size") || !strings.Contains(err.Error(), "640x640, 864x480") {
		t.Errorf("512x512 for V2 HD: %v", err)
	}
	if err := CheckOffered(acct, "v2", Resolution{Width: 384, Height: 640}); err == nil || !strings.Contains(err.Error(), "offered: 512x512") {
		t.Errorf("384x640 for v2: %v", err)
	}
	if err := CheckOffered(&Account{}, "v2pro", Resolution{Width: 544, Height: 736}); err != nil {
		t.Errorf("no list: %v", err)
	}
}

// A V2 HD create is a v2 create with engine v2hd (sent as v2pro): the candidate options
// and the preview line limit apply as for v2.
func TestManifestV2Pro(t *testing.T) {
	m, err := (CreateInput{Name: "x", Engine: "v2pro", Image: "https://x/a.png", Voice: "https://x/a.m4a", Candidates: 2,
		SeedSelection: "auto", Resolution: &Resolution{Width: 864, Height: 480}}).Manifest()
	if err != nil || m["engine"] != "v2pro" {
		t.Fatalf("%v %v", m, err)
	}
	if r, _ := m["resolution"].(map[string]int); r["width"] != 864 || r["height"] != 480 {
		t.Fatalf("resolution %v", m["resolution"])
	}
	if _, err := (CreateInput{Name: "x", Engine: "v2pro", Candidates: 2}).Manifest(); err == nil {
		t.Error("a V2 HD draft with candidates: no error")
	}
	if _, err := (CreateInput{Name: "x", Engine: "v2pro", PreviewMessage: strings.Repeat("a", MaxPrimeTextChars+1)}).Manifest(); err == nil {
		t.Error("a V2 HD preview line over the limit: no error")
	}
	if _, err := (CreateInput{Name: "x", Engine: "v2", Resolution: &Resolution{Width: 640, Height: 640}}).Manifest(); err == nil {
		t.Error("a V2 HD size on v2: no error")
	}
}

// A clip still rendering when generate clip's own wait runs out names
// clip-status with the call's key, so it can be followed without resending.
func TestGenerateClipPointsAtClipStatusWhenItStopsWaiting(t *testing.T) {
	var key string
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		w.Header().Set("Retry-After", strconv.Itoa(int(2*generationBudget/time.Second)))
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"request_in_progress","message":"still running"}`))
	})
	_, err := c.GenerateClip(context.Background(), ClipInput{Image: "upload:a", Prompt: "waves"}, "")
	if err == nil || key == "" || !strings.Contains(err.Error(), "atmee generate clip-status "+key+" --wait") ||
		!strings.Contains(err.Error(), "--idempotency-key "+key) {
		t.Fatalf("key %q: %v", key, err)
	}
}

// WaitClip gives up at its timeout with a rendering clip, naming how to
// check again; it never waits past the timeout.
func TestWaitClipTimesOut(t *testing.T) {
	var reads atomic.Int32
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requestId":"` + aid + `","status":"rendering","retryAfterSeconds":60}`))
	})
	st, err := c.WaitClip(context.Background(), aid, 30*time.Second, time.Second, nil)
	if err == nil || st == nil || st.Status != ClipRendering || reads.Load() != 1 || !strings.Contains(err.Error(), "clip-status "+aid) {
		t.Fatalf("%v %v, %d reads", st, err, reads.Load())
	}
}

// A read still running at WaitClip's timeout is cut off there, not left to
// the HTTP client's own timeout, and answers that the clip is still rendering.
func TestWaitClipBoundsAReadByItsTimeout(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	start := time.Now()
	_, err := c.WaitClip(context.Background(), aid, 200*time.Millisecond, time.Second, nil)
	if err == nil || !strings.Contains(err.Error(), "still rendering") || time.Since(start) > 3*time.Second {
		t.Fatalf("after %s: %v", time.Since(start), err)
	}
}
