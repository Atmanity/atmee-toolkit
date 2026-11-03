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
	"strings"
	"testing"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/creds"
)

const (
	testKeyID  = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	testSecret = "sk_atmee_0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab"
)

const testKeyJSON = `{"id":"` + testKeyID + `","name":"stream app","keyPrefix":"sk_atmee_01234567","lastFour":"89ab",` +
	`"scopes":["sessions","avatars:read"],"active":true,"maxSessionSeconds":900,"maxConcurrentSessions":3,` +
	`"spendCapMicrocredits":5000,"spendCapWindow":"daily","visitorMemory":false,"createdAt":"2026-10-22T00:00:00Z"}`

// keysServer answers the key routes like the API; status/body override.
func keysServer(t *testing.T, status int, errBody string) *[]seen {
	t.Helper()
	home := isolate(t)
	var reqs []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		s := seen{method: r.Method, path: r.URL.Path}
		_ = json.Unmarshal(raw, &s.body)
		reqs = append(reqs, s)
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(errBody))
			return
		}
		switch {
		case r.Method == http.MethodGet:
			revoked := strings.Replace(strings.Replace(testKeyJSON, `"active":true`, `"active":false,"revokedAt":"2026-10-22T01:00:00Z"`, 1),
				testKeyID, "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee", 1)
			_, _ = w.Write([]byte(`{"apiKeys":[` + testKeyJSON + `,` + revoked + `]}`))
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(strings.Replace(testKeyJSON, `"active":true`, `"active":false`, 1)))
		default:
			if r.Method == http.MethodPost && r.URL.Path == "/v1/api_keys" {
				w.WriteHeader(http.StatusCreated)
			}
			_, _ = w.Write([]byte(`{"apiKey":"` + testSecret + `","key":` + testKeyJSON + `}`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	// Keys are managed signed in, never with a key.
	if err := (creds.Store{Dir: home}).SaveSession(creds.Session{BaseURL: srv.URL, AccessToken: "user-token", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	return &reqs
}

func TestKeysListHidesRevokedUnlessAll(t *testing.T) {
	keysServer(t, 0, "")
	out, errOut, code := run(t, "", "keys", "list")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if !strings.Contains(out, "stream app  "+testKeyID) || strings.Contains(out, "REVOKED") ||
		!strings.Contains(out, "sk_atmee_01234567…89ab") || !strings.Contains(out, "15m0s · 3 at once") {
		t.Fatalf("list:\n%s", out)
	}
	out, _, _ = run(t, "", "keys", "list", "--all")
	if !strings.Contains(out, "REVOKED 2026-10-2") {
		t.Fatalf("--all:\n%s", out)
	}
}

// --out writes the secret into the env file (other lines kept, an old
// value replaced, owner-only) and never prints it.
func TestKeysCreateWritesTheSecretToOut(t *testing.T) {
	reqs := keysServer(t, 0, "")
	env := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(env, []byte("# app\nOTHER=1\nexport ATMEE_API_KEY=sk_atmee_old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := run(t, "", "keys", "create", "--name", "stream app", "--scopes", "avatars:read",
		"--avatar", conductorAvatar, "--max-session", "15m", "--spend-cap", "50", "--spend-cap-window", "daily", "--out", env)
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if strings.Contains(out+errOut, testSecret) || !strings.Contains(out, "its secret is in "+env) {
		t.Fatalf("the secret was printed or not reported:\n%s%s", out, errOut)
	}
	got, _ := os.ReadFile(env)
	if string(got) != "# app\nOTHER=1\nATMEE_API_KEY="+testSecret+"\n" {
		t.Fatalf(".env:\n%s", got)
	}
	if fi, _ := os.Stat(env); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	want := map[string]any{"name": "stream app", "scopes": []any{"avatars:read"}, "avatarId": conductorAvatar,
		"maxSessionSeconds": 900.0, "spendCapMicrocredits": 5000.0, "spendCapWindow": "daily"}
	if b, w := toJSONString((*reqs)[0].body), toJSONString(want); b != w || (*reqs)[0].path != "/v1/api_keys" {
		t.Fatalf("sent %s, want %s", b, w)
	}
}

func TestKeysCreateWithoutOutPrintsOnceAndWarns(t *testing.T) {
	keysServer(t, 0, "")
	out, errOut, code := run(t, "", "keys", "create", "--name", "x")
	if code != 0 || !strings.Contains(out, "ATMEE_API_KEY="+testSecret) || !strings.Contains(errOut, "shown only now") {
		t.Fatalf("%d\n%s%s", code, out, errOut)
	}
}

func TestKeysRotateToANewFileAndJSONLeavesTheSecretOut(t *testing.T) {
	reqs := keysServer(t, 0, "")
	env := filepath.Join(t.TempDir(), "app.env")
	out, errOut, code := run(t, "", "keys", "rotate", testKeyID, "--out", env, "--env-var", "STREAM_KEY", "--json")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if strings.Contains(out, testSecret) || jsonOf(t, out)["id"] != testKeyID {
		t.Fatalf("--json with --out:\n%s", out)
	}
	if got, _ := os.ReadFile(env); string(got) != "STREAM_KEY="+testSecret+"\n" {
		t.Fatalf("file: %q", got)
	}
	if r := (*reqs)[0]; r.method != http.MethodPost || r.path != "/v1/api_keys/"+testKeyID+"/rotate" {
		t.Fatalf("sent %s %s", r.method, r.path)
	}
}

func TestKeysRevokeAsksFirst(t *testing.T) {
	reqs := keysServer(t, 0, "")
	if _, _, code := run(t, "n\n", "keys", "revoke", testKeyID); code != 2 || len(*reqs) != 0 {
		t.Fatalf("no: %d %d", code, len(*reqs))
	}
	out, errOut, code := run(t, "", "keys", "revoke", testKeyID, "--yes")
	if code != 0 || !strings.Contains(out, "Revoked stream app") || (*reqs)[0].method != http.MethodDelete {
		t.Fatalf("%d %s %s", code, out, errOut)
	}
}

func TestKeysUsageErrorsSendNothing(t *testing.T) {
	reqs := keysServer(t, 0, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"create"}, "--name is required"},
		{[]string{"create", "--name", "x", "--scopes", "admin"}, "unknown scope \"admin\""},
		{[]string{"create", "--name", "x", "--avatar", "nope"}, "is not an avatar id"},
		{[]string{"create", "--name", "x", "--max-session", "10s"}, "--max-session takes 30s to 4h"},
		{[]string{"create", "--name", "x", "--max-concurrent", "0"}, "--max-concurrent takes 1 to 100"},
		{[]string{"create", "--name", "x", "--spend-cap", "0"}, "--spend-cap takes a positive"},
		{[]string{"create", "--name", "x", "--spend-cap", "0.001"}, "--spend-cap takes a positive"},
		{[]string{"create", "--name", "x", "--out", "/"}, "is a directory"},
		{[]string{"rotate", testKeyID, "--out", "/no/such/dir/.env"}, "cannot be written"},
		{[]string{"create", "--name", "x", "--spend-cap-window", "weekly"}, "daily, monthly or total"},
		{[]string{"create", "--name", "x", "--out", ".env", "--env-var", "1BAD"}, "is not a variable name"},
		{[]string{"rotate"}, "missing KEY_ID (find it with `atmee keys list`)"},
		{[]string{"create", "--name", "x", "--env-var", "1BAD"}, "is not a variable name"},
		{[]string{"rotate", testKeyID, "--env-var", ""}, "--env-var is empty"},
		{[]string{"revoke", "nope", "--yes"}, "is not an API key id"},
	} {
		_, errOut, code := run(t, "", append([]string{"keys"}, tc.args...)...)
		if code != 2 || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: %d %s", tc.args, code, errOut)
		}
	}
	if len(*reqs) != 0 {
		t.Fatalf("sent %d requests", len(*reqs))
	}
}

// An API key cannot manage keys: the CLI says which key is in the way
// before sending anything, and the API's own refusal reaches the user too.
func TestKeysWithAnAPIKeyIsRefused(t *testing.T) {
	reqs := keysServer(t, 0, "")
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	if _, errOut, code := run(t, "", "keys", "list"); code != 2 || !strings.Contains(errOut, "unset ATMEE_API_KEY") || len(*reqs) != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	keysServer(t, http.StatusForbidden, `{"error":"owner_or_admin_required","message":"only the account's owner or an admin manages its API keys"}`)
	if _, errOut, code := run(t, "", "keys", "list"); code != 1 || !strings.Contains(errOut, "owner or an admin") {
		t.Fatalf("%d %s", code, errOut)
	}
}

// --out goes through a symlink to its target and replaces a spaced
// assignment; --json without --out still warns.
func TestKeysOutFollowsALinkAndJSONWarns(t *testing.T) {
	keysServer(t, 0, "")
	dir := t.TempDir()
	target := filepath.Join(dir, "real.env")
	if err := os.WriteFile(target, []byte("ATMEE_API_KEY = sk_atmee_old\r\nOTHER=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".env")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := run(t, "", "keys", "rotate", testKeyID, "--out", link); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced by a file")
	}
	if got, _ := os.ReadFile(target); string(got) != "ATMEE_API_KEY="+testSecret+"\nOTHER=1\n" {
		t.Fatalf("target: %q", got)
	}
	out, errOut, code := run(t, "", "keys", "create", "--name", "x", "--json")
	if code != 0 || jsonOf(t, out)["apiKey"] != testSecret || !strings.Contains(errOut, "shown only now") {
		t.Fatalf("%d %s %s", code, out, errOut)
	}
}

func toJSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func envValue(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "ATMEE_API_KEY="); ok {
			return v
		}
	}
	t.Fatalf("no ATMEE_API_KEY in %s", b)
	return ""
}

// A link to a file that does not exist yet creates the target.
func TestKeysOutThroughADanglingLink(t *testing.T) {
	keysServer(t, 0, "")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "secrets"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".env")
	if err := os.Symlink("secrets/app.env", link); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := run(t, "", "keys", "create", "--name", "x", "--out", link); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced by a file")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "secrets", "app.env")); string(got) != "ATMEE_API_KEY="+testSecret+"\n" {
		t.Fatalf("target: %q", got)
	}
}

// An --out that exists but cannot be read stops the command before a key
// is created or rotated.
func TestKeysUnreadableOutSendsNothing(t *testing.T) {
	reqs := keysServer(t, 0, "")
	env := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(env, []byte("OTHER=1\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(env); err == nil {
		t.Skip("running as a user who reads every file")
	}
	if _, errOut, code := run(t, "", "keys", "rotate", testKeyID, "--out", env); code != 2 || !strings.Contains(errOut, "cannot be read") || len(*reqs) != 0 {
		t.Fatalf("%d %s, %d requests", code, errOut, len(*reqs))
	}
}

type brokenAfter struct{ after int }

func (f *brokenAfter) Write(p []byte) (int, error) {
	if f.after <= 0 {
		return 0, errors.New("disk full")
	}
	f.after--
	return len(p), nil
}

// When the printed secret cannot be written, the command fails and says
// how to get a new one.
func TestKeysCreateFailsWhenTheSecretCannotBePrinted(t *testing.T) {
	keysServer(t, 0, "")
	var errOut bytes.Buffer
	code := Execute([]string{"keys", "create", "--name", "x"}, &brokenAfter{after: 7}, &errOut, strings.NewReader(""))
	if code != 1 || !strings.Contains(errOut.String(), "printing its secret failed") || !strings.Contains(errOut.String(), "atmee keys rotate") {
		t.Fatalf("%d %s", code, errOut.String())
	}
}
