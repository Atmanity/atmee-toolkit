package cmd

import (
	"context"
	"errors"
	"fmt"
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
			"A login that belongs to several accounts acts for the Personal one, unless you pick another with --account " +
			"(or ATMEE_ACCOUNT_ID). Signing in again as the same user keeps the account you picked; --account \"\" goes back " +
			"to the default. Accounts are shown by id, and by name where the API returns one. The sign-in is for " +
			"the Avatar API at ATMEE_BASE_URL; signing in to another one replaces it.\n\n" +
			"--api-key saves an Avatar API key instead (prompted, hidden; create one in Atmee Studio -> API keys). " +
			"ATMEE_API_KEY, when set, wins over both.",
		Example: `  atmee login
  atmee login --api-key`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if apiKey {
				return a.saveAPIKey(jsonOut)
			}
			return a.browserLogin(account, cmd.Flags().Changed("account"), noBrowser, jsonOut)
		},
	}
	c.Flags().BoolVar(&apiKey, "api-key", false, "Save an API key (prompted) instead of signing in with the browser")
	c.Flags().StringVar(&account, "account", "", "Account id to act for, when your login belongs to several (default: the Personal one, or the one picked before; \"\" forgets that pick)")
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

// keptAccount is the account a new sign-in keeps when it names none: the one
// picked for the saved sign-in, if that was the same user on the same API.
// Another user's pick would fail every call with 403 account_not_allowed.
func keptAccount(prev *creds.Session, base, accessToken string) string {
	if prev == nil || !sameBase(prev.BaseURL, base) {
		return ""
	}
	if u := oauth.Subject(accessToken); u == "" || u != oauth.Subject(prev.AccessToken) {
		return ""
	}
	return prev.AccountID
}

// dropRefused forgets the saved account if the API refuses it to this user
// (it would fail every call), and reports whether it did.
func (a *app) dropRefused() (bool, error) {
	cred, err := a.sessionCredential()
	if err != nil {
		return false, err
	}
	// The account just saved, not ATMEE_ACCOUNT_ID: that one wins over it only
	// while it is set.
	if u, ok := cred.(*userSession); ok {
		u.ignoreEnv = true
	}
	cl, err := a.client(cred)
	if err != nil {
		return false, err
	}
	c, cancel := context.WithTimeout(ctx(), whoamiTimeout)
	defer cancel()
	_, err = cl.CurrentKey(c)
	var apiErr *api.Error
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodeAccountNotAllowed {
		return false, nil // allowed, or not to be told now
	}
	s := a.store.Session()
	if s == nil {
		return false, nil
	}
	s.AccountID = ""
	return true, a.store.SaveSession(*s)
}

// useAccount saves the account a login names, once the API takes it. An
// account it refuses, or a name that means none of the user's, is not saved:
// the sign-in stays, on the default account.
func (a *app) useAccount(account string) error {
	id := account
	if a.resolveAccount != nil {
		var err error
		if id, err = a.resolveAccount(account); err != nil {
			return fmt.Errorf("%w\nYou are signed in, on the default account: no account was saved", err)
		}
	}
	s := a.store.Session()
	if s == nil {
		return errors.New("not signed in: run `atmee login`")
	}
	s.AccountID = id
	if err := a.store.SaveSession(*s); err != nil {
		return err
	}
	dropped, err := a.dropRefused()
	if err != nil {
		return err
	}
	if dropped {
		return fmt.Errorf("this login may not act for the account %q. You are signed in, on the default account: no account was saved", account)
	}
	return nil
}

func (a *app) browserLogin(account string, accountGiven, noBrowser, jsonOut bool) error {
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
	account = strings.TrimSpace(account)
	kept := false
	if !accountGiven {
		account = keptAccount(a.store.Session(), base, t.AccessToken)
		kept = account != ""
	}
	sess := creds.Session{
		BaseURL: base, TokenEndpoint: srv.Token, ClientID: clientID, Resource: srv.Resource,
		AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresAt: t.ExpiresAt,
		Email: oauth.Email(t.AccessToken),
	}
	if kept {
		sess.AccountID = account
	}
	if err := a.store.SaveSession(sess); err != nil {
		return err
	}
	var refused error
	if accountGiven && account != "" {
		refused = a.useAccount(account)
	}
	if kept {
		dropped, err := a.dropRefused()
		if err != nil {
			return err
		}
		if dropped {
			a.note("This login may no longer act for the account picked before (%s): back to the default.\n", account)
		} else if !jsonOut {
			a.note("Kept the account picked before (%s). To go back to the default: `atmee login --account \"\"`.\n", account)
		}
	}
	// Ask the API who this is, which also settles the account.
	cur, lookupErr := a.currentKey()
	var apiErr *api.Error
	if errors.As(lookupErr, &apiErr) && apiErr.Code == api.CodeAccountAmbiguous {
		a.note("Your login belongs to several accounts and the API could not tell which one is Personal: " +
			"run `atmee login --account <account id>`, or set ATMEE_ACCOUNT_ID.\n")
		a.noteAccounts(lookupErr)
	}
	if a.store.EnvAPIKey() != "" && !jsonOut {
		a.note("ATMEE_API_KEY is set: the commands use that key, not this sign-in.\n")
	}
	if jsonOut {
		out := map[string]any{"signed_in": true, "email": sess.Email, "base_url": base}
		if cur != nil {
			out["user"] = cur
			out["account_source"] = a.accountSource()
			if facts := a.accountJSON(); facts != nil {
				out["account"] = facts
			}
		} else if lookupErr != nil {
			out["user_error"] = a.rename(lookupErr.Error())
			if choices := ambiguousAccounts(lookupErr); len(choices) > 0 {
				out["accounts"] = choices
			}
		}
		if refused != nil {
			out["account_error"] = a.rename(refused.Error())
		}
		if err := a.printJSON(out); err != nil {
			return err
		}
		return refused
	}
	who := sess.Email
	if who == "" {
		who = "your Atmee login"
	}
	a.printf("Signed in as %s -> %s\n", who, base)
	if cur != nil {
		a.printf("Account      %s\n", cur.AccountId)
		a.printAccountUsed()
	} else if lookupErr != nil && (apiErr == nil || apiErr.Code != api.CodeAccountAmbiguous) {
		a.note("Could not check the sign-in with the API (%s).\n", a.rename(describeLookup(lookupErr)))
		if h := a.accountHint(lookupErr); h != "" {
			a.note(strings.ReplaceAll(h, "%", "%%") + "\n")
		}
	}
	return refused
}

func (a *app) logoutCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "logout",
		Short: "Sign out: remove the saved sign-in or API key.",
		Long: "Sign out: remove the saved sign-in or API key from this machine. The sign-in itself stays listed in your account settings " +
			"on atmee.ai (Connected agents, or Connected apps) until you revoke it there.",
		Args: cobra.NoArgs,
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
