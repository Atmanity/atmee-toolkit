package cmd

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/creds"
)

// fakeAuth is an Avatar API with its authorization server: it registers
// clients, answers the authorization page with a redirect, trades codes and
// refresh tokens, and checks the bearer token on /v1/api_keys/current.
type fakeAuth struct {
	srv *httptest.Server
	mu  sync.Mutex
	// state of the one login in flight
	redirect, challenge string
	access, refresh     string
	refreshes           int
	// several makes the user a member of two accounts.
	several bool
	// lastAccount is the X-Account-Id of the last API call.
	lastAccount string
	// wrongState answers the authorization page with another state.
	wrongState bool
	// user is the sub and email of the next sign-in ("u-1" when empty).
	user string
	// denied is an account the user may no longer act for (403).
	denied string
	// noRefresh issues access tokens without a refresh token.
	noRefresh bool
}

func jwtFor(user string, n int) string {
	email := "dev@example.com"
	if user == "" {
		user = "u-1"
	} else {
		email = user + "@example.com"
	}
	p, _ := json.Marshal(map[string]any{"email": email, "sub": user, "n": n})
	return "e30." + base64.RawURLEncoding.EncodeToString(p) + ".sig"
}

func newFakeAuth(t *testing.T) *fakeAuth {
	f := &fakeAuth{}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	base := f.srv.URL
	js := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{"resource": base + "/mcp", "authorization_servers": []string{base + "/auth/v1"}})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/auth/v1", func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{
			"authorization_endpoint": base + "/auth/v1/oauth/authorize",
			"token_endpoint":         base + "/auth/v1/oauth/token",
			"registration_endpoint":  base + "/auth/v1/oauth/clients/register",
		})
	})
	mux.HandleFunc("POST /auth/v1/oauth/clients/register", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			RedirectURIs []string `json:"redirect_uris"`
			AuthMethod   string   `json:"token_endpoint_auth_method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if len(b.RedirectURIs) != 1 || !strings.HasPrefix(b.RedirectURIs[0], "http://127.0.0.1:") || b.AuthMethod != "none" {
			js(w, 400, map[string]any{"error": "invalid_client_metadata"})
			return
		}
		f.mu.Lock()
		f.redirect = b.RedirectURIs[0]
		f.mu.Unlock()
		js(w, 201, map[string]any{"client_id": "cli-1"})
	})
	mux.HandleFunc("GET /auth/v1/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.mu.Lock()
		defer f.mu.Unlock()
		if q.Get("client_id") != "cli-1" || q.Get("redirect_uri") != f.redirect || q.Get("code_challenge_method") != "S256" ||
			q.Get("resource") != base+"/mcp" {
			http.Error(w, "bad authorize request", 400)
			return
		}
		f.challenge = q.Get("code_challenge")
		state := q.Get("state")
		if f.wrongState {
			state = "someone-else"
		}
		http.Redirect(w, r, f.redirect+"?code=c1&state="+url.QueryEscape(state), http.StatusFound)
	})
	mux.HandleFunc("POST /auth/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Form.Get("resource") != base+"/mcp" {
			js(w, 400, map[string]any{"error": "invalid_target"})
			return
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if r.Form.Get("code") != "c1" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge ||
				r.Form.Get("redirect_uri") != f.redirect || r.Form.Get("client_id") != "cli-1" {
				js(w, 400, map[string]any{"error": "invalid_grant"})
				return
			}
		case "refresh_token":
			if r.Form.Get("refresh_token") != f.refresh || r.Form.Get("client_id") != "cli-1" {
				js(w, 400, map[string]any{"error": "invalid_grant"})
				return
			}
			f.refreshes++
		default:
			js(w, 400, map[string]any{"error": "unsupported_grant_type"})
			return
		}
		f.access = jwtFor(f.user, f.refreshes)
		f.refresh = fmt.Sprintf("r%d", f.refreshes+1)
		answer := map[string]any{"access_token": f.access, "refresh_token": f.refresh, "expires_in": 3600, "token_type": "bearer"}
		if f.noRefresh {
			delete(answer, "refresh_token")
		}
		js(w, 200, answer)
	})
	mux.HandleFunc("GET /v1/api_keys/current", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+f.access || f.access == "" {
			js(w, 401, map[string]any{"error": "unauthorized", "message": "invalid token"})
			return
		}
		f.lastAccount = r.Header.Get("X-Account-Id")
		acct := f.lastAccount
		if acct != "" && acct == f.denied {
			js(w, 403, map[string]any{"error": "account_not_allowed", "message": "not that account"})
			return
		}
		if acct == "" {
			if f.several {
				js(w, 409, map[string]any{"error": "account_ambiguous", "message": "send X-Account-Id",
					"accounts": []map[string]any{{"id": "acct-1", "username": "ada"}, {"id": "acct-2"}}})
				return
			}
			acct = "acct-1"
		}
		js(w, 200, map[string]any{"kind": "user", "userId": "u-1", "accountId": acct})
	})
	return f
}

// browseFollowing is a browser that loads the page and follows its redirect
// back to the CLI.
func browseFollowing(t *testing.T) {
	t.Helper()
	old := browse
	browse = func(u string) error {
		go func() {
			if resp, err := http.Get(u); err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	t.Cleanup(func() { browse = old })
}

func TestBrowserLoginSignsInAndActsAsTheUser(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)

	out, errOut, code := run(t, "", "login")
	if code != 0 || !strings.Contains(out, "Signed in as dev@example.com") || !strings.Contains(out, "acct-1") {
		t.Fatalf("login: %d %q %q", code, out, errOut)
	}
	if !strings.Contains(errOut, "/auth/v1/oauth/authorize?") {
		t.Fatalf("the sign-in link is printed for a browser that did not open: %q", errOut)
	}
	s := (creds.Store{Dir: home}).Session()
	if s == nil || s.ClientID != "cli-1" || s.RefreshToken != "r1" || s.BaseURL != f.srv.URL {
		t.Fatalf("saved session: %+v", s)
	}
	out, _, code = run(t, "", "whoami")
	if code != 0 || !strings.Contains(out, "Signed in as dev@example.com") || !strings.Contains(out, "Signed-in user u-1 on account acct-1") {
		t.Fatalf("whoami: %d %q", code, out)
	}
	run(t, "", "logout")
	if out, _, _ = run(t, "", "whoami", "--json"); !strings.Contains(out, `"signed_in": false`) {
		t.Fatalf("logout signs out: %s", out)
	}
}

func TestExpiredSignInIsRefreshedAndSaved(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	if _, errOut, code := run(t, "", "login"); code != 0 {
		t.Fatal(errOut)
	}
	st := creds.Store{Dir: home}
	s := st.Session()
	s.ExpiresAt = time.Now().Add(-time.Minute)
	if err := st.SaveSession(*s); err != nil {
		t.Fatal(err)
	}
	if out, errOut, code := run(t, "", "whoami"); code != 0 || !strings.Contains(out, "acct-1") {
		t.Fatalf("whoami after expiry: %d %q %q", code, out, errOut)
	}
	if s = st.Session(); f.refreshes != 1 || s.RefreshToken != "r2" || time.Until(s.ExpiresAt) < 30*time.Minute {
		t.Fatalf("refresh: %d %+v", f.refreshes, s)
	}
}

func TestSignInOfSeveralAccountsNamesOne(t *testing.T) {
	isolate(t)
	f := newFakeAuth(t)
	f.several = true
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	if _, errOut, code := run(t, "", "login"); code != 0 || !strings.Contains(errOut, "--account") {
		t.Fatalf("an ambiguous login says how to pick: %d %q", code, errOut)
	}
	if out, _, code := run(t, "", "login", "--account", "acct-2"); code != 0 || !strings.Contains(out, "acct-2") || f.lastAccount != "acct-2" {
		t.Fatalf("--account: %d %q %q", code, out, f.lastAccount)
	}
	t.Setenv("ATMEE_ACCOUNT_ID", "acct-3")
	if out, _, _ := run(t, "", "whoami"); !strings.Contains(out, "acct-3") {
		t.Fatalf("ATMEE_ACCOUNT_ID wins: %q", out)
	}
}

func TestSignInIsForOneAPI(t *testing.T) {
	isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	if _, errOut, code := run(t, "", "login"); code != 0 {
		t.Fatal(errOut)
	}
	t.Setenv("ATMEE_BASE_URL", "https://api.example.invalid")
	if _, errOut, code := run(t, "", "balance"); code != 1 || !strings.Contains(errOut, "run `atmee login` again for this API") {
		t.Fatalf("a sign-in is not sent to another API: %d %q", code, errOut)
	}
	// An API key, when set, wins over the sign-in.
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_0123456789abcdef")
	if out, _, _ := run(t, "", "whoami", "--json"); !strings.Contains(out, `"api_key": true`) {
		t.Fatalf("ATMEE_API_KEY wins: %s", out)
	}
}

func TestLoginRefusesAForeignAnswer(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	f.wrongState = true
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	if _, errOut, code := run(t, "", "login"); code != 1 || !strings.Contains(errOut, "did not match this login") {
		t.Fatalf("state mismatch: %d %q", code, errOut)
	}
	if (creds.Store{Dir: home}).Session() != nil {
		t.Fatal("nothing is saved")
	}
}

func TestLoginWithoutBrowserSignInSaysUseAKey(t *testing.T) {
	isolate(t) // ATMEE_BASE_URL is unreachable
	if _, errOut, code := run(t, "", "login"); code != 1 || !strings.Contains(errOut, "atmee login --api-key") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestSignInWithoutRefreshTokenLastsUntilItExpires(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	f.noRefresh = true
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	if _, errOut, code := run(t, "", "login"); code != 0 {
		t.Fatal(errOut)
	}
	if out, _, code := run(t, "", "whoami"); code != 0 || !strings.Contains(out, "acct-1") {
		t.Fatalf("a sign-in without a refresh token is kept: %d %q", code, out)
	}
	st := creds.Store{Dir: home}
	s := st.Session()
	s.ExpiresAt = time.Now().Add(30 * time.Second)
	_ = st.SaveSession(*s)
	if out, errOut, code := run(t, "", "whoami"); code != 0 || !strings.Contains(out, "acct-1") {
		t.Fatalf("a token close to expiry is still used: %d %q %q", code, out, errOut)
	}
	s.ExpiresAt = time.Now().Add(-time.Minute)
	_ = st.SaveSession(*s)
	if _, errOut, code := run(t, "", "balance"); code != 1 || !strings.Contains(errOut, "run `atmee login` again") {
		t.Fatalf("expired without a refresh token: %d %q", code, errOut)
	}
}

func savedAccount(t *testing.T, home string) string {
	t.Helper()
	s := (creds.Store{Dir: home}).Session()
	if s == nil {
		t.Fatal("no saved sign-in")
	}
	return s.AccountID
}

func TestSigningInAgainKeepsTheAccountOfTheSameUser(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	if _, errOut, code := run(t, "", "login", "--account", "acct-2"); code != 0 {
		t.Fatal(errOut)
	}
	out, errOut, code := run(t, "", "login")
	if code != 0 || savedAccount(t, home) != "acct-2" || f.lastAccount != "acct-2" || !strings.Contains(out, "Account      acct-2\n") {
		t.Fatalf("the same user keeps the account: %d %q %q", code, out, errOut)
	}
	if !strings.Contains(errOut, "Kept the account picked before (acct-2)") || !strings.Contains(out, "Picked by    `atmee login --account`") {
		t.Fatalf("and is told: %q %q", out, errOut)
	}
	if run(t, "", "login", "--account", "acct-3"); savedAccount(t, home) != "acct-3" {
		t.Fatalf("--account overrides: %q", savedAccount(t, home))
	}
	out, errOut, code = run(t, "", "login", "--account", "")
	if code != 0 || savedAccount(t, home) != "" || f.lastAccount != "" || !strings.Contains(out, "Picked by    default") {
		t.Fatalf("--account \"\" goes back to the default: %d %q %q", code, out, errOut)
	}
	if strings.Contains(errOut, "Kept") {
		t.Fatalf("nothing was kept: %q", errOut)
	}
}

func TestSigningInAsAnotherUserDropsTheAccount(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	run(t, "", "login", "--account", "acct-2")
	f.user = "u-2"
	out, errOut, code := run(t, "", "login")
	if code != 0 || savedAccount(t, home) != "" || f.lastAccount != "" || strings.Contains(errOut, "Kept") {
		t.Fatalf("another user's account is dropped: %d %q %q", code, out, errOut)
	}
	// The same user on another API is a different sign-in too.
	run(t, "", "login", "--account", "acct-2")
	g := newFakeAuth(t)
	g.user = "u-2"
	t.Setenv("ATMEE_BASE_URL", g.srv.URL)
	if _, errOut, code = run(t, "", "login"); code != 0 || savedAccount(t, home) != "" {
		t.Fatalf("another API's account is dropped: %d %q", code, errOut)
	}
	// Tokens that name no user can't be told apart.
	if a := keptAccount(&creds.Session{BaseURL: "https://x", AccessToken: "opaque", AccountID: "acct-2"}, "https://x", "opaque"); a != "" {
		t.Fatalf("kept %q for an unknown user", a)
	}
}

func TestAKeptAccountNoLongerAllowedIsDropped(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	run(t, "", "login", "--account", "acct-2")
	f.denied = "acct-2"
	out, errOut, code := run(t, "", "login")
	if code != 0 || savedAccount(t, home) != "" || !strings.Contains(out, "acct-1") || !strings.Contains(errOut, "back to the default") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	// Also while ATMEE_ACCOUNT_ID names another account, which stays in use.
	f.denied = ""
	run(t, "", "login", "--account", "acct-2")
	f.denied = "acct-2"
	t.Setenv("ATMEE_ACCOUNT_ID", "acct-4")
	if out, _, _ = run(t, "", "login"); savedAccount(t, home) != "" || !strings.Contains(out, "acct-4") {
		t.Fatalf("with ATMEE_ACCOUNT_ID: %q %q", savedAccount(t, home), out)
	}
}

// An account the API refuses is not saved, and login says so and fails; the
// sign-in itself stays, on the default account.
func TestLoginSavesNoAccountTheAPIRefuses(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	f.denied = "acct-9"
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	run(t, "", "login", "--account", "acct-2")
	out, errOut, code := run(t, "", "login", "--account", "acct-9")
	if code != 1 || savedAccount(t, home) != "" || !strings.Contains(out, "Signed in as") || !strings.Contains(out, "Account      acct-1\n") ||
		!strings.Contains(errOut, `this login may not act for the account "acct-9". You are signed in, on the default account: no account was saved`) {
		t.Fatalf("%d %q %q %q", code, out, errOut, savedAccount(t, home))
	}
	if out, errOut, code = run(t, "", "whoami"); code != 0 || !strings.Contains(out, "acct-1") {
		t.Fatalf("later calls work: %d %q %q", code, out, errOut)
	}
	out, _, code = run(t, "", "login", "--account", "acct-9", "--json")
	var j map[string]any
	if err := json.Unmarshal([]byte(out), &j); err != nil || code != 1 || j["signed_in"] != true || j["account_error"] == nil || j["user"] == nil {
		t.Fatalf("--json: %d %v %s", code, err, out)
	}
	// Also while ATMEE_ACCOUNT_ID or a key is in use: what is saved is checked.
	t.Setenv("ATMEE_ACCOUNT_ID", "acct-4")
	if _, _, code = run(t, "", "login", "--account", "acct-9"); code != 1 || savedAccount(t, home) != "" {
		t.Fatalf("%d %q", code, savedAccount(t, home))
	}
	if _, errOut, code = run(t, "", "login", "--account", "acct-2"); code != 0 || savedAccount(t, home) != "acct-2" {
		t.Fatalf("a good one is saved: %d %q", code, errOut)
	}
}

func TestARefusedAccountVariableSaysWhatItTakes(t *testing.T) {
	isolate(t)
	f := newFakeAuth(t)
	f.denied = "My Account"
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	run(t, "", "login")
	t.Setenv("ATMEE_ACCOUNT_ID", "My Account")
	out, errOut, code := run(t, "", "whoami")
	if code != 1 || !strings.Contains(out, "Signed in as") || strings.Contains(errOut, "X-Account-Id") ||
		!strings.Contains(errOut, `this login may not act for the account "My Account"; ATMEE_ACCOUNT_ID takes the id of an account you belong to`) {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	// login still signs in, and says it too.
	if _, errOut, code = run(t, "", "login"); code != 0 || !strings.Contains(errOut, "ATMEE_ACCOUNT_ID takes the id") {
		t.Fatalf("login: %d %q", code, errOut)
	}
	t.Setenv("ATMEE_ACCOUNT_ID", "")
	f.denied = "acct-2"
	s := (creds.Store{Dir: os.Getenv("ATMEE_HOME")}).Session()
	s.AccountID = "acct-2"
	_ = (creds.Store{Dir: os.Getenv("ATMEE_HOME")}).SaveSession(*s)
	if _, errOut, code = run(t, "", "whoami"); code != 1 || !strings.Contains(errOut, "`atmee login --account \"\"` goes back to the default account") {
		t.Fatalf("saved: %d %q", code, errOut)
	}
}

// A firewall's page is one line, whatever the command and in --json too.
func TestAWebPageIsNeverPrinted(t *testing.T) {
	page := "<!DOCTYPE html>\n<html><title>Attention Required! | Cloudflare</title>" + strings.Repeat("<p>blocked</p>", 500) + "</html>"
	status := http.StatusForbidden
	signedIn(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(page))
	})
	_, errOut, code := run(t, "", "balance")
	if code != 1 || errOut != "Error: 403: the server answered with a web page, not the API; a firewall may block this path\n" {
		t.Fatalf("%d %q", code, errOut)
	}
	out, _, code := run(t, "", "whoami", "--json")
	if code != 0 || !json.Valid([]byte(out)) || strings.Contains(out, "<") || !strings.Contains(out, "web page") {
		t.Fatalf("whoami --json: %d %q", code, out)
	}
	for _, status = range []int{http.StatusBadGateway, http.StatusOK} {
		if _, errOut, code = run(t, "", "account"); code != 1 || strings.Contains(errOut, "<") || !strings.Contains(errOut, "web page, not the API") || strings.Contains(errOut, "firewall") {
			t.Fatalf("%d: %d %q", status, code, errOut)
		}
	}
	for _, status = range []int{http.StatusForbidden, http.StatusServiceUnavailable, http.StatusOK} {
		_, errOut, code = run(t, "", "login")
		if code != 1 || strings.Contains(errOut, "<") || !strings.Contains(errOut, "sign-in got a web page instead of the API") || !strings.Contains(errOut, "a firewall or a proxy may block it") ||
			strings.Contains(errOut, "API key") || strings.Contains(errOut, "does not offer") {
			t.Fatalf("login, %d: %d %q", status, code, errOut)
		}
	}
}

// So does the authorization server's metadata, and an API that has no browser
// sign-in still says to use a key.
func TestSignInMetadataBehindAFirewall(t *testing.T) {
	isolate(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-protected-resource" {
			_, _ = fmt.Fprintf(w, `{"resource":"x","authorization_servers":[%q]}`, srv.URL+"/auth/v1")
			return
		}
		// The first place is blocked; the later ones are not there.
		if !strings.HasPrefix(r.URL.Path, "/.well-known/oauth-authorization-server") {
			w.WriteHeader(http.StatusNotFound)
			if !strings.HasSuffix(r.URL.Path, "openid-configuration") {
				_, _ = w.Write([]byte("<html>no such page</html>"))
			}
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>blocked</html>"))
	}))
	defer srv.Close()
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	if _, errOut, code := run(t, "", "login"); code != 1 || !strings.Contains(errOut, "(403 from ") || !strings.Contains(errOut, "firewall") || strings.Contains(errOut, "API key") {
		t.Fatalf("%d %q", code, errOut)
	}
	none := httptest.NewServer(http.NotFoundHandler())
	defer none.Close()
	t.Setenv("ATMEE_BASE_URL", none.URL)
	if _, errOut, code := run(t, "", "login"); code != 1 || !strings.Contains(errOut, "does not offer browser sign-in") || !strings.Contains(errOut, "use an API key") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestAnAmbiguousAccountListsTheChoices(t *testing.T) {
	isolate(t)
	f := newFakeAuth(t)
	f.several = true
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	want := "  acct-1  @ada\n  acct-2\n"
	if _, errOut, code := run(t, "", "login"); code != 0 || !strings.Contains(errOut, want) {
		t.Fatalf("login: %d %q", code, errOut)
	}
	if _, errOut, code := run(t, "", "whoami"); code != 0 || !strings.Contains(errOut, want) {
		t.Fatalf("whoami: %d %q", code, errOut)
	}
	out, _, _ := run(t, "", "whoami", "--json")
	var w struct {
		KeyError string `json:"key_error"`
		Accounts []struct{ ID, Username string }
	}
	if err := json.Unmarshal([]byte(out), &w); err != nil || w.KeyError == "" || len(w.Accounts) != 2 || w.Accounts[0].Username != "ada" {
		t.Fatalf("whoami --json: %v %s", err, out)
	}
}

// Any command refused for the account lists the choices under its error.
func TestARefusedCommandListsTheAccounts(t *testing.T) {
	isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	browseFollowing(t)
	run(t, "", "login")
	f.srv.Config.Handler.(*http.ServeMux).HandleFunc("GET /v1/account/balance", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"account_ambiguous","message":"name one","accounts":[{"id":"acct-9","username":"lin"}]}`))
	})
	out, errOut, code := run(t, "", "balance", "--json")
	if code != 1 || out != "" || !strings.Contains(errOut, "Error: 409 account_ambiguous: name one\n") || !strings.Contains(errOut, "  acct-9  @lin\n") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
}

func TestAKeyMakesTheAccountVariableCountForNothing(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	t.Setenv("ATMEE_BASE_URL", accountAPI(t).URL)
	if _, errOut, _ := run(t, "", "balance"); strings.Contains(errOut, "Warning") {
		t.Fatalf("nothing is ignored: %q", errOut)
	}
	t.Setenv("ATMEE_ACCOUNT_ID", "acct-3")
	out, errOut, code := run(t, "", "account")
	if code != 0 || strings.Count(errOut, "Warning: ATMEE_ACCOUNT_ID is ignored, because ATMEE_API_KEY is set") != 1 || strings.Contains(out, "Warning") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	if !strings.HasPrefix(out, "Account 11111111-1111-4111-8111-111111111111\nPicked by    the API key") {
		t.Fatalf("account: %q", out)
	}
	if _, errOut, _ = run(t, "", "account", "--json"); errOut != "" {
		t.Fatalf("--json stays quiet: %q", errOut)
	}
	out, _, _ = run(t, "", "whoami", "--json")
	if !strings.Contains(out, `"account_source": "api_key"`) {
		t.Fatalf("whoami --json: %s", out)
	}
}

// A credentials file an older atmee wrote still loads, its account included.
func TestAnOlderSignInStillLoads(t *testing.T) {
	home := isolate(t)
	f := newFakeAuth(t)
	t.Setenv("ATMEE_BASE_URL", f.srv.URL)
	f.access = jwtFor("", 0)
	b, _ := json.Marshal(map[string]any{"oauth": map[string]any{"base_url": f.srv.URL, "client_id": "cli-1", "token_endpoint": f.srv.URL,
		"access_token": f.access, "expires_at": time.Now().Add(time.Hour), "account_id": "acct-2"}})
	if err := os.WriteFile(filepath.Join(home, "credentials.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, errOut, code := run(t, "", "whoami"); code != 0 || !strings.Contains(out, "Signed-in user u-1 on account acct-2\n") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	browseFollowing(t)
	if run(t, "", "login"); savedAccount(t, home) != "acct-2" {
		t.Fatalf("and the account is kept: %q", savedAccount(t, home))
	}
}
