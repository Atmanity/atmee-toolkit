package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Atmanity/toolkit/cli/internal/creds"
)

const testSession = "11111111-2222-4333-8444-555555555555"

// signedIn points the CLI at h as a signed-in user.
func signedIn(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	home := isolate(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if err := (creds.Store{Dir: home}).SaveSession(creds.Session{BaseURL: srv.URL, AccessToken: "user-token", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
}

// A call still running answers 409 request_in_progress; the same key is sent
// again and pays once.
func TestGenerateWaitsWithTheSameKey(t *testing.T) {
	var calls atomic.Int32
	var keys []string
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"error":"request_in_progress","message":"still running"}`)
			return
		}
		_, _ = io.WriteString(w, `{"portraits":[{"upload":"upload:abc","expiresAt":"2026-11-02T00:00:00Z","preview":"","provider":"p1","unavailable":false}],`+
			`"microcreditsCharged":100,"replayed":false,"failedProviders":["p2"]}`)
	})
	out, errOut, code := run(t, "", "generate", "portraits", "--description", "a lighthouse keeper")
	if code != 0 || len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("%d %s %v", code, errOut, keys)
	}
	if !strings.Contains(out, "--image upload:abc") || !strings.Contains(out, "1.00 credits charged") || !strings.Contains(errOut, "No result from: p2") {
		t.Fatalf("%q %q", out, errOut)
	}
}

// A lost answer is never sent again: the error hands back the key.
func TestGenerateTransportFailureGivesTheKey(t *testing.T) {
	var calls atomic.Int32
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		c, _, _ := w.(http.Hijacker).Hijack()
		_ = c.Close()
	})
	_, errOut, code := run(t, "", "generate", "voice", "--description", "a calm warm alto, unhurried")
	if code != 1 || calls.Load() != 1 || !strings.Contains(errOut, "--idempotency-key ") {
		t.Fatalf("%d %d %s", code, calls.Load(), errOut)
	}
}

// A proxy's 5xx can follow a charge, so it hands back the key; an API error the
// server coded did not reach one.
func TestGenerateServerFailureGivesTheKeyOnlyWhenUncoded(t *testing.T) {
	for _, c := range []struct {
		name, body string
		status     int
		key        bool
	}{
		{"proxy", "<html>Bad Gateway</html>", http.StatusBadGateway, true},
		{"timeout", "", http.StatusGatewayTimeout, true},
		{"coded", `{"error":"generation_failed","message":"no"}`, http.StatusBadGateway, false},
		{"refused", `{"error":"insufficient_credits","message":"no"}`, http.StatusPaymentRequired, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			calls := 0
			signedIn(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(c.status)
				_, _ = io.WriteString(w, c.body)
			})
			_, errOut, code := run(t, "", "generate", "portraits", "--description", "a lighthouse keeper")
			if code == 0 || calls != 1 || strings.Contains(errOut, "--idempotency-key") != c.key {
				t.Fatalf("code %d, %d calls: %s", code, calls, errOut)
			}
		})
	}
}

func TestGenerateRefusesBadInputBeforeSending(t *testing.T) {
	signedIn(t, func(http.ResponseWriter, *http.Request) { t.Error("sent") })
	for _, args := range [][]string{
		{"generate", "portraits"},
		{"generate", "enhance"},
		{"generate", "enhance", "--image", "http://x/y.png"},
		{"generate", "voice", "--description", "too short"},
		{"generate", "voice", "--description", "a calm warm alto, unhurried", "--language", "xx"},
		{"generate", "clip", "--prompt", "waves"},
		{"generate", "clip", "--image", "upload:a"},
		{"generate", "clip", "--image", "http://x/y.png", "--prompt", "waves"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--audio", "http://x/y.wav", "--duration", "6"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--audio", "upload:b"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--duration", "16"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--duration", "0"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--resolution", "wide"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--seed", "-1"},
		{"generate", "clip", "--image", "./me.png", "--prompt", "waves", "--idempotency-key", "k"},
		{"generate", "clip", "--image", "upload:a", "--prompt", "waves", "--audio", "./a.wav", "--duration", "6", "--idempotency-key", "k"},
		{"avatars", "delete-version", testSession, "nothex", "--yes"},
	} {
		if _, _, code := run(t, "", args...); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}

// A clip sends what was asked and nothing else, with the same key while it
// runs, and shows what it was charged.
func TestGenerateClip(t *testing.T) {
	var calls atomic.Int32
	var body string
	var keys []string
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/generation/clips" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"error":"request_in_progress","message":"still running"}`)
			return
		}
		_, _ = io.WriteString(w, `{"clip":{"upload":"upload:clip","url":"https://store.example/clip.mp4","expiresAt":"2026-11-02T00:00:00Z",`+
			`"durationSeconds":6,"width":384,"height":672,"unavailable":false},"microcreditsCharged":100,"replayed":false}`)
	})
	out, errOut, code := run(t, "", "generate", "clip", "--image", "upload:a", "--prompt", "a slow nod", "--audio", "https://x.example/a.wav",
		"--duration", "6", "--resolution", "384x672", "--seed", "0")
	if code != 0 || len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("%d %s %v", code, errOut, keys)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"image": "upload:a", "prompt": "a slow nod", "audio": "https://x.example/a.wav", "durationSeconds": float64(6),
		"resolution": map[string]any{"width": float64(384), "height": float64(672)}, "seed": float64(0)}
	if !reflect.DeepEqual(sent, want) {
		t.Fatalf("sent %v", sent)
	}
	if !strings.Contains(out, "upload:clip") || !strings.Contains(out, "https://store.example/clip.mp4") || !strings.Contains(out, "1.00 credits charged") {
		t.Fatalf("%q", out)
	}
}

// A silent clip leaves the length, size and seed to the API's defaults.
func TestGenerateClipWithoutV2AccessSaysHowToGetIt(t *testing.T) {
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"v2_access_required","message":"invite only"}`)
	})
	_, errOut, code := run(t, "", "generate", "clip", "--image", "https://x.example/a.png", "--prompt", "waves")
	if code == 0 || !strings.Contains(errOut, "v2_access_required") || !strings.Contains(errOut, "`atmee redeem CODE`") {
		t.Fatalf("%d %s", code, errOut)
	}
}

func TestGenerateClipDefaults(t *testing.T) {
	var body string
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"clip":{"upload":"upload:clip","expiresAt":"2026-11-02T00:00:00Z","durationSeconds":8,"width":512,"height":512,"unavailable":true},`+
			`"microcreditsCharged":0,"replayed":true}`)
	})
	out, errOut, code := run(t, "", "generate", "clip", "--image", "upload:a", "--prompt", "a slow nod")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if body != `{"image":"upload:a","prompt":"a slow nod"}` {
		t.Fatalf("sent %s", body)
	}
	if !strings.Contains(out, "expired") || !strings.Contains(out, "replayed") {
		t.Fatalf("%q", out)
	}
}

// Local files go up first, each as its kind, and a lost answer hands both
// uploads back with the key, so a retry sends the same inputs.
func TestGenerateClipLocalFilesAndLostAnswer(t *testing.T) {
	ids := map[string]string{"portrait": "3f1c2a9e-8d4b-4f6a-9c2e-1b7d5a0e4c11", "voice": "4f1c2a9e-8d4b-4f6a-9c2e-1b7d5a0e4c12"}
	var srv *httptest.Server
	var mu sync.Mutex
	puts := map[string]string{}
	home := isolate(t)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads":
			var in struct{ Kind string }
			_ = json.Unmarshal(b, &in)
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"uploadId":"`+ids[in.Kind]+`","kind":"`+in.Kind+`","status":"pending","uploadUrl":"`+srv.URL+`/put/`+in.Kind+`","uploadHeaders":{},`+
				`"maxBytes":10485760,"allowedTypes":[],"expiresAt":"2026-11-02T00:00:00Z"}`)
		case r.Method == http.MethodPut:
			mu.Lock()
			puts[r.URL.Path] = r.Header.Get("Content-Type")
			mu.Unlock()
		case r.Method == http.MethodGet:
			id := strings.TrimPrefix(r.URL.Path, "/v1/uploads/")
			_, _ = io.WriteString(w, `{"uploadId":"`+id+`","kind":"portrait","status":"uploaded","expiresAt":"2026-11-02T00:00:00Z"}`)
		default:
			c, _, _ := w.(http.Hijacker).Hijack()
			_ = c.Close()
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if err := (creds.Store{Dir: home}).SaveSession(creds.Session{BaseURL: srv.URL, AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	photo, audio := filepath.Join(dir, "me.png"), filepath.Join(dir, "line.wav")
	for _, p := range []string{photo, audio} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, errOut, code := run(t, "", "generate", "clip", "--image", photo, "--prompt", "waves", "--audio", audio, "--duration", "6")
	if code != 1 || !strings.Contains(errOut, "--idempotency-key ") ||
		!strings.Contains(errOut, "--image upload:"+ids["portrait"]) || !strings.Contains(errOut, "--audio upload:"+ids["voice"]) {
		t.Fatalf("%d %s", code, errOut)
	}
	if puts["/put/portrait"] != "image/png" || puts["/put/voice"] != "audio/wav" {
		t.Fatalf("puts: %v", puts)
	}
}

// A generated upload goes where a file does: the JSON form of a replace, and
// the assets of a create.
func TestUploadRefsReachReplaceAndCreate(t *testing.T) {
	var got []string
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Content-Type")+" "+string(b))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"avatarId":"`+testSession+`","kind":"conversational","status":"building","statusUrl":"/x"}`)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	})
	ref := "upload:3f1c2a9e-8d4b-4f6a-9c2e-1b7d5a0e4c11"
	if _, errOut, code := run(t, "", "avatars", "replace-appearance", testSession, ref); code != 0 {
		t.Fatal(errOut)
	}
	if _, errOut, code := run(t, "", "avatars", "replace-voice", testSession, ref); code != 0 {
		t.Fatal(errOut)
	}
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "A", "--image", ref, "--voice", ref, "--engine", "v2", "--json"); code != 0 {
		t.Fatal(errOut)
	}
	if len(got) != 3 || !strings.Contains(got[0], `application/json {"image":"`+ref) || !strings.Contains(got[1], `{"voice":"`+ref) ||
		!strings.Contains(got[2], `"assets":{"image":{"url":"`+ref) {
		t.Fatalf("%q", got)
	}
}

func TestKeysUpdateCanClearEveryOptionalScope(t *testing.T) {
	reqs := keysServer(t, 0, "")
	if _, errOut, code := run(t, "", "keys", "update", testKeyID, "--scopes="); code != 0 {
		t.Fatal(errOut)
	}
	if got, ok := (*reqs)[0].body["scopes"].([]any); !ok || len(got) != 0 {
		t.Fatalf("sent %v", (*reqs)[0].body)
	}
}

func TestKeysUpdate(t *testing.T) {
	reqs := keysServer(t, 0, "")
	out, errOut, code := run(t, "", "keys", "update", testKeyID, "--name", "next", "--scopes", "avatars:read,generation:write",
		"--any-avatar", "--max-session", "15m", "--max-concurrent", "2", "--spend-cap", "12.5", "--spend-cap-window", "total", "--visitor-memory=false")
	if code != 0 || !strings.Contains(out, "as saved") {
		t.Fatalf("%d %s %s", code, out, errOut)
	}
	r := (*reqs)[0]
	b := r.body
	avatar, widened := b["avatarId"]
	if r.method != http.MethodPatch || r.path != "/v1/api_keys/"+testKeyID || b["name"] != "next" || !widened || avatar != nil ||
		b["maxSessionSeconds"] != float64(900) || b["maxConcurrentSessions"] != float64(2) || b["spendCapMicrocredits"] != float64(1250) ||
		b["spendCapWindow"] != "total" || b["visitorMemory"] != false {
		t.Fatalf("%s %s %v", r.method, r.path, b)
	}
	before := len(*reqs)
	for _, args := range [][]string{
		{"keys", "update", testKeyID},
		{"keys", "update", "nope", "--name", "x"},
		{"keys", "update", testKeyID, "--name", " "},
		{"keys", "update", testKeyID, "--scopes", "root"},
		{"keys", "update", testKeyID, "--any-avatar", "--avatar", testSession},
	} {
		if _, _, code := run(t, "", args...); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if len(*reqs) != before {
		t.Fatalf("a bad update was sent: %v", (*reqs)[before:])
	}
}

func TestUploadOverTheLimitIsRefusedBeforeSending(t *testing.T) {
	var puts atomic.Int32
	signedIn(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts.Add(1)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"uploadId":"11111111-1111-4111-8111-111111111111","uploadUrl":"http://`+r.Host+`/put",`+
			`"pageUrl":"https://x.example/u","maxBytes":1048576,"allowedTypes":["image/png"],"expiresAt":"2026-11-02T00:00:00Z"}`)
	})
	photo := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(photo, make([]byte, 1<<20+1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := run(t, "", "generate", "enhance", "--image", photo)
	if code == 0 || puts.Load() != 0 || !strings.Contains(errOut, "over the 1 MB limit") {
		t.Fatalf("%d %d %s", code, puts.Load(), errOut)
	}
}

func TestEnhanceUploadsALocalFile(t *testing.T) {
	var srv *httptest.Server
	uploadID := "3f1c2a9e-8d4b-4f6a-9c2e-1b7d5a0e4c11"
	var put, enhance string
	home := isolate(t)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"uploadId":"`+uploadID+`","kind":"portrait","status":"pending","uploadUrl":"`+srv.URL+`/put","uploadHeaders":{"X-Sig":"s"},`+
				`"maxBytes":10485760,"allowedTypes":["image/png"],"expiresAt":"2026-11-02T00:00:00Z"}`)
		case r.Method == http.MethodPut:
			put = r.Header.Get("Content-Type") + " " + r.Header.Get("X-Sig") + " " + string(b)
		case r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{"uploadId":"`+uploadID+`","kind":"portrait","status":"uploaded","expiresAt":"2026-11-02T00:00:00Z"}`)
		default:
			enhance = string(b)
			_, _ = io.WriteString(w, `{"portraits":[{"upload":"upload:out","expiresAt":"2026-11-02T00:00:00Z","preview":"","provider":"p","unavailable":false}],`+
				`"microcreditsCharged":100,"replayed":true,"failedProviders":[]}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if err := (creds.Store{Dir: home}).SaveSession(creds.Session{BaseURL: srv.URL, AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(t.TempDir(), "me.png")
	if err := os.WriteFile(photo, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := run(t, "", "generate", "enhance", "--image", photo, "--prompt", "as a detective")
	if code != 0 || put != "image/png s png" || !strings.Contains(enhance, `"image":"upload:`+uploadID) || !strings.Contains(enhance, "as a detective") {
		t.Fatalf("%d %s %q %q", code, errOut, put, enhance)
	}
	if !strings.Contains(out, "replayed") {
		t.Fatalf("%s", out)
	}
}

// A retry must send the same inputs: a lost answer hands back the upload with
// the key, and a retry from the local file is refused.
func TestEnhanceRetryKeepsTheUpload(t *testing.T) {
	uploadID := "3f1c2a9e-8d4b-4f6a-9c2e-1b7d5a0e4c11"
	var srv *httptest.Server
	home := isolate(t)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"uploadId":"`+uploadID+`","kind":"portrait","status":"pending","uploadUrl":"`+srv.URL+`/put","uploadHeaders":{},`+
				`"maxBytes":10485760,"allowedTypes":["image/png"],"expiresAt":"2026-11-02T00:00:00Z"}`)
		case r.Method == http.MethodPut:
		case r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{"uploadId":"`+uploadID+`","kind":"portrait","status":"uploaded","expiresAt":"2026-11-02T00:00:00Z"}`)
		default:
			c, _, _ := w.(http.Hijacker).Hijack()
			_ = c.Close()
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if err := (creds.Store{Dir: home}).SaveSession(creds.Session{BaseURL: srv.URL, AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(t.TempDir(), "me.png")
	if err := os.WriteFile(photo, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := run(t, "", "generate", "enhance", "--image", photo)
	if code != 1 || !strings.Contains(errOut, "--idempotency-key ") || !strings.Contains(errOut, "--image upload:"+uploadID) {
		t.Fatalf("%d %s", code, errOut)
	}
	if _, errOut, code := run(t, "", "generate", "enhance", "--image", photo, "--idempotency-key", "k"); code != 2 || !strings.Contains(errOut, "upload:ID") {
		t.Fatalf("%d %s", code, errOut)
	}
}
