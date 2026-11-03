package cmd

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	// noRefresh issues access tokens without a refresh token.
	noRefresh bool
}

func jwtWithEmail(email string, n int) string {
	p, _ := json.Marshal(map[string]any{"email": email, "n": n})
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
		f.access = jwtWithEmail("dev@example.com", f.refreshes)
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
		if acct == "" {
			if f.several {
				js(w, 409, map[string]any{"error": "account_ambiguous", "message": "send X-Account-Id"})
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
