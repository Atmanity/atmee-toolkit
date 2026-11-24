// Package oauth signs a user in to the Avatar API with the browser, the way
// an MCP client does: the API names its authorization server
// (/.well-known/oauth-protected-resource), the CLI registers itself there as
// a public client for one loopback redirect, sends the user to the
// authorization page with PKCE, takes the code on that redirect and trades it
// for tokens. The access token is short-lived; Refresh trades the refresh
// token for a new pair.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/webpage"
)

// Server is the authorization server the Avatar API trusts.
type Server struct {
	// Resource is the protected resource the tokens are for.
	Resource      string
	Authorization string
	Token         string
	Registration  string
}

// Tokens is one token answer.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Discover reads the Avatar API's protected-resource metadata and its
// authorization server's metadata.
func Discover(ctx context.Context, hc *http.Client, baseURL string) (*Server, error) {
	var pr struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := getJSON(ctx, hc, strings.TrimRight(baseURL, "/")+"/.well-known/oauth-protected-resource", &pr); err != nil {
		// A web page says nothing of what the API offers, and an API key
		// would meet it too.
		var page *webpage.Error
		if errors.As(err, &page) {
			return nil, blocked(page)
		}
		return nil, fmt.Errorf("this Avatar API does not offer browser sign-in (%w); use an API key: atmee login --api-key", err)
	}
	if len(pr.AuthorizationServers) == 0 {
		return nil, errors.New("this Avatar API names no authorization server; use an API key: atmee login --api-key")
	}
	issuer := strings.TrimRight(pr.AuthorizationServers[0], "/")
	var md struct {
		Authorization string `json:"authorization_endpoint"`
		Token         string `json:"token_endpoint"`
		Registration  string `json:"registration_endpoint"`
	}
	var err error
	var page *webpage.Error // the first web page met, whatever came after
	for _, u := range metadataURLs(issuer) {
		if err = getJSON(ctx, hc, u, &md); err == nil && md.Authorization != "" && md.Token != "" {
			break
		}
		if page == nil {
			errors.As(err, &page)
		}
	}
	if md.Authorization == "" || md.Token == "" {
		if page != nil {
			return nil, blocked(page)
		}
		return nil, fmt.Errorf("reading the authorization server's metadata: %v", err)
	}
	if md.Registration == "" {
		return nil, errors.New("the authorization server does not register clients; use an API key: atmee login --api-key")
	}
	return &Server{Resource: pr.Resource, Authorization: md.Authorization, Token: md.Token, Registration: md.Registration}, nil
}

// blocked is a sign-in that met a web page instead of the API.
func blocked(page *webpage.Error) error {
	return fmt.Errorf("sign-in got a web page instead of the API (%d from %s): a firewall or a proxy may block it", page.Status, page.Where)
}

// metadataURLs are where an issuer with a path keeps its metadata: RFC 8414's
// well-known prefix before the path first, then OpenID's suffix.
func metadataURLs(issuer string) []string {
	u, err := url.Parse(issuer)
	if err != nil || u.Path == "" || u.Path == "/" {
		return []string{issuer + "/.well-known/oauth-authorization-server", issuer + "/.well-known/openid-configuration"}
	}
	root := u.Scheme + "://" + u.Host
	return []string{
		root + "/.well-known/oauth-authorization-server" + u.Path,
		issuer + "/.well-known/oauth-authorization-server",
		issuer + "/.well-known/openid-configuration",
	}
}

// Login runs the browser sign-in. open is given the authorization URL (and
// may fail: the URL is also handed to show, for the user to open by hand).
// It returns the registered client id with the tokens; refreshing needs it.
func Login(ctx context.Context, hc *http.Client, s *Server, open func(string) error, show func(string)) (string, *Tokens, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("listening for the sign-in redirect: %w", err)
	}
	defer ln.Close()
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)

	clientID, err := register(ctx, hc, s.Registration, redirect)
	if err != nil {
		return "", nil, err
	}
	verifier, state := random(32), random(16)
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	if s.Resource != "" {
		q.Set("resource", s.Resource)
	}
	authURL := s.Authorization + "?" + q.Encode()
	show(authURL)
	_ = open(authURL)

	type result struct {
		code string
		err  error
	}
	got := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		p := r.URL.Query()
		var res result
		switch {
		case p.Get("state") != state:
			res.err = errors.New("the sign-in answer did not match this login (state); run atmee login again")
		case p.Get("error") != "":
			res.err = fmt.Errorf("sign-in refused: %s %s", p.Get("error"), p.Get("error_description"))
		case p.Get("code") == "":
			res.err = errors.New("the sign-in answer carried no code")
		default:
			res.code = p.Get("code")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.err != nil {
			fmt.Fprintf(w, "<p>Atmee sign-in failed: %s</p>", html.EscapeString(res.err.Error()))
		} else {
			fmt.Fprint(w, "<p>Signed in to Atmee. You can close this tab and go back to the terminal.</p>")
		}
		select {
		case got <- res:
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	var res result
	select {
	case res = <-got:
	case <-ctx.Done():
		return "", nil, fmt.Errorf("waiting for the browser sign-in: %w", ctx.Err())
	}
	if res.err != nil {
		return "", nil, res.err
	}
	t, err := token(ctx, hc, s.Token, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {res.code},
		"redirect_uri":  {redirect},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}, s.Resource)
	return clientID, t, err
}

// Refresh trades a refresh token for a new pair, for the same resource. The
// server may rotate the refresh token; when it sends none back, the old one
// stays good.
func Refresh(ctx context.Context, hc *http.Client, tokenURL, clientID, refreshToken, resource string) (*Tokens, error) {
	t, err := token(ctx, hc, tokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}, resource)
	if err != nil {
		return nil, err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

// Email is the email claim of a JWT access token, for display only (the
// token is not verified here; the API does that).
func Email(accessToken string) string { return claims(accessToken).Email }

// Subject is the token's user id, "" when it has none. Like Email, it is
// read without checking the signature: good for telling two sign-ins apart,
// never for trusting one.
func Subject(accessToken string) string { return claims(accessToken).Subject }

type tokenClaims struct {
	Email   string `json:"email"`
	Subject string `json:"sub"`
}

func claims(accessToken string) (c tokenClaims) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return c
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	return c
}

func register(ctx context.Context, hc *http.Client, endpoint, redirect string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"client_name":                "Atmee CLI",
		"redirect_uris":              []string{redirect},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := do(hc, req, &out); err != nil {
		return "", fmt.Errorf("registering the CLI with the sign-in server: %w", err)
	}
	if out.ClientID == "" {
		return "", errors.New("registering the CLI with the sign-in server: no client_id in the answer")
	}
	return out.ClientID, nil
}

// token posts a token request. resource, when set, is the RFC 8707 resource
// indicator the authorization request named: a server that binds the code or
// the grant to it expects it here too.
func token(ctx context.Context, hc *http.Client, endpoint string, form url.Values, resource string) (*Tokens, error) {
	if resource != "" {
		form.Set("resource", resource)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := do(hc, req, &out); err != nil {
		return nil, fmt.Errorf("getting a token: %w", err)
	}
	if out.AccessToken == "" {
		return nil, errors.New("getting a token: no access_token in the answer")
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Tokens{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, ExpiresAt: time.Now().Add(ttl)}, nil
}

func getJSON(ctx context.Context, hc *http.Client, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return do(hc, req, v)
}

func do(hc *http.Client, req *http.Request, v any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
			Message     string `json:"msg"`
		}
		_ = json.Unmarshal(b, &e)
		msg := strings.TrimSpace(e.Error + " " + e.Description + " " + e.Message)
		if msg == "" {
			if webpage.Is(resp.Header.Get("Content-Type"), b) {
				return &webpage.Error{Where: req.URL.Host + req.URL.Path, Status: resp.StatusCode}
			}
			msg = strings.TrimSpace(string(b))
		}
		return fmt.Errorf("%s: %d %s", req.URL.Host+req.URL.Path, resp.StatusCode, msg)
	}
	if webpage.Is(resp.Header.Get("Content-Type"), b) {
		return &webpage.Error{Where: req.URL.Host + req.URL.Path, Status: resp.StatusCode}
	}
	return json.Unmarshal(b, v)
}

func random(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
