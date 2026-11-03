package cmd

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/creds"
	"github.com/Atmanity/atmee-toolkit/cli/internal/oauth"
	"github.com/Atmanity/atmee-toolkit/cli/internal/web"
)

// loginTimeout bounds the wait for the browser sign-in; tests shorten it.
var loginTimeout = 5 * time.Minute

// browse opens the sign-in page; tests replace it with a scripted browser.
var browse = web.Browse

func maskKey(k string) string {
	if len(k) <= 13 {
		return "…"
	}
	return k[:9] + "…" + k[len(k)-4:]
}

func (a *app) loginCmd() *cobra.Command {
	var jsonOut, apiKey, noBrowser bool
	var account string
	c := &cobra.Command{
		Use:   "login",
		Short: "Sign in with your browser (or save an API key with --api-key).",
		Long: "Sign in with the browser, as the MCP server's sign-in does: the commands then act as you, on your account. " +
			"A login that belongs to several accounts picks one with --account (or ATMEE_ACCOUNT_ID). The sign-in is for " +
			"the Avatar API at ATMEE_BASE_URL; signing in to another one replaces it.\n\n" +
			"--api-key saves an Avatar API key instead (prompted, hidden; create one in Atmee Studio -> API keys). Sponsor " +
			"commands need a key. ATMEE_API_KEY, when set, wins over both.",
		Example: `  atmee login
  atmee login --api-key`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if apiKey {
				return a.saveAPIKey(jsonOut)
			}
			return a.browserLogin(account, noBrowser, jsonOut)
		},
	}
	c.Flags().BoolVar(&apiKey, "api-key", false, "Save an API key (prompted) instead of signing in with the browser")
	c.Flags().StringVar(&account, "account", "", "Account id to act for, when your login belongs to several")
	c.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the sign-in link instead of opening the browser")
	c.Flags().BoolVar(&jsonOut, "json", false, "Print the result as JSON")
	return c
}

func (a *app) saveAPIKey(jsonOut bool) error {
	key, err := a.prompt("API key (sk_atmee_...)", true)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(key, "sk_atmee_") && !strings.HasPrefix(key, "sk_live_") {
		return errors.New("that does not look like an Atmee API key")
	}
	if err := a.store.SaveAPIKey(key); err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(map[string]any{"api_key": true, "saved": maskKey(key)})
	}
	a.printf("Saved API key %s\n", maskKey(key))
	return nil
}

func (a *app) browserLogin(account string, noBrowser, jsonOut bool) error {
	base := a.baseURL()
	hc := &http.Client{Timeout: 30 * time.Second}
	c, cancel := context.WithTimeout(ctx(), loginTimeout)
	defer cancel()
	srv, err := oauth.Discover(c, hc, base)
	if err != nil {
		return err
	}
	open := browse
	if noBrowser {
		open = func(string) error { return nil }
	}
	clientID, t, err := oauth.Login(c, hc, srv, open, func(u string) {
		if noBrowser {
			a.note("Open this link to sign in to Atmee:\n  %s\n", u)
		} else {
			a.note("Opening the browser to sign in to Atmee. If it does not open, visit:\n  %s\n", u)
		}
	})
	if err != nil {
		return err
	}
	sess := creds.Session{
		BaseURL: base, TokenEndpoint: srv.Token, ClientID: clientID, Resource: srv.Resource,
		AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresAt: t.ExpiresAt,
		Email: oauth.Email(t.AccessToken), AccountID: strings.TrimSpace(account),
	}
	if err := a.store.SaveSession(sess); err != nil {
		return err
	}
	// Ask the API who this is, which also settles the account.
	cur, lookupErr := a.currentKey()
	var apiErr *api.Error
	if errors.As(lookupErr, &apiErr) && apiErr.Code == "account_ambiguous" {
		a.note("Your login belongs to several accounts: run `atmee login --account <account id>`, or set ATMEE_ACCOUNT_ID.\n")
	}
	if jsonOut {
		out := map[string]any{"signed_in": true, "email": sess.Email, "base_url": base}
		if cur != nil {
			out["user"] = cur
		} else if lookupErr != nil {
			out["user_error"] = lookupErr.Error()
		}
		return a.printJSON(out)
	}
	who := sess.Email
	if who == "" {
		who = "your Atmee login"
	}
	a.printf("Signed in as %s -> %s\n", who, base)
	if cur != nil {
		a.printf("Account      %s\n", cur.AccountId)
	} else if lookupErr != nil && apiErr == nil {
		a.note("Could not check the sign-in with the API (%s).\n", describeLookup(lookupErr))
	}
	return nil
}

func (a *app) logoutCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "logout",
		Short: "Sign out: remove the saved sign-in or API key.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.store.Remove(); err != nil {
				return err
			}
			a.printf("Signed out: removed the saved sign-in or API key.\n")
			return nil
		},
	}
	c.Flags().Bool("api-key", true, "") // older spelling, same effect
	_ = c.Flags().MarkHidden("api-key")
	return c
}
