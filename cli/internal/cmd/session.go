package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/creds"
	"github.com/Atmanity/atmee-toolkit/cli/internal/oauth"
)

// credential is how the avatar commands authenticate: ATMEE_API_KEY or the
// saved key, else the saved browser sign-in.
func (a *app) credential() (api.Credential, error) {
	if k := a.store.APIKey(); k != "" {
		a.warnAccountIgnored()
		return api.APIKey(k), nil
	}
	named, err := a.namedAccount()
	if err != nil {
		return nil, err
	}
	cred, err := a.sessionCredential()
	if u, ok := cred.(*userSession); ok {
		u.named = named
	}
	return cred, err
}

// namedAccount is the id of the account this call names (accountFlag), ""
// for none. It may ask the API, so it comes before the call's own credential.
func (a *app) namedAccount() (string, error) {
	if a.accountFlag == "" || a.resolveAccount == nil {
		return "", nil
	}
	if _, err := a.sessionCredential(); err != nil {
		return "", err
	}
	id, err := a.resolveAccount(a.accountFlag)
	if err != nil {
		return "", err
	}
	a.accountFlag = id // resolved once
	return id, nil
}

// sessionCredential is the saved browser sign-in, whatever API key is set.
func (a *app) sessionCredential() (api.Credential, error) {
	s := a.store.Session()
	if s == nil {
		return nil, errors.New("not signed in: run `atmee login`, or set ATMEE_API_KEY (create one in Atmee Studio -> API keys)")
	}
	if !sameBase(s.BaseURL, a.baseURL()) {
		return nil, fmt.Errorf("signed in to %s, but ATMEE_BASE_URL is %s: run `atmee login` again for this API", s.BaseURL, a.baseURL())
	}
	return &userSession{store: a.store, s: *s, hc: &http.Client{Timeout: 30 * time.Second}}, nil
}

func sameBase(a, b string) bool { return strings.TrimRight(a, "/") == strings.TrimRight(b, "/") }

// accountID is the account a user of several acts for: ATMEE_ACCOUNT_ID, else
// the one chosen at login, else "" (the API picks).
func accountID(s creds.Session) string {
	if v := creds.EnvAccountID(); v != "" {
		return v
	}
	return s.AccountID
}

// Where the account the commands act for comes from, as --json names it.
const (
	accountFromKey     = "api_key"
	accountFromFlag    = "flag"
	accountFromEnv     = "env"
	accountFromLogin   = "login"
	accountFromDefault = "default"
)

func (a *app) accountSource() string {
	switch s := a.store.Session(); {
	case a.store.APIKey() != "":
		return accountFromKey
	case a.accountFlag != "":
		return accountFromFlag
	case creds.EnvAccountID() != "":
		return accountFromEnv
	case s != nil && s.AccountID != "":
		return accountFromLogin
	}
	return accountFromDefault
}

// accountDetails is the account the commands act for, read for accountFacts
// alone: nil without that hook, or when it can't be read.
func (a *app) accountDetails() *api.Account {
	if a.accountFacts == nil {
		return nil
	}
	cl, err := a.api()
	if err != nil {
		return nil
	}
	c, cancel := context.WithTimeout(ctx(), whoamiTimeout)
	defer cancel()
	x, err := cl.Account(c)
	if err != nil {
		return nil
	}
	return x
}

// accountJSON is what --json adds about an account, nil for nothing.
func (a *app) accountJSON() map[string]any {
	x := a.accountDetails()
	if x == nil {
		return nil
	}
	_, facts := a.accountFacts(x)
	if facts == nil {
		return nil
	}
	out := map[string]any{"id": x.AccountId, "plan": x.Plan}
	for k, v := range facts {
		out[k] = v
	}
	return out
}

// printAccountSource says, under an account's id, how the account in use was
// picked, and that an id is all the API returns (or, from x, what it returns).
func (a *app) printAccountSource(x *api.Account) {
	by := "default: your only account, or your Personal account when you have several"
	switch a.accountSource() {
	case accountFromKey:
		by = "the API key"
	case accountFromFlag:
		by = "--account"
	case accountFromEnv:
		by = "ATMEE_ACCOUNT_ID"
	case accountFromLogin:
		by = "`atmee login --account`"
		if a.savedBy != "" {
			by = a.savedBy
		}
	}
	var lines []string
	if x != nil && a.accountFacts != nil {
		lines, _ = a.accountFacts(x)
	}
	if len(lines) == 0 {
		a.printf("Picked by    " + by + " (shown by id: the API returns no account name)\n")
		return
	}
	a.printf("Picked by    " + by + "\n")
	for _, l := range lines {
		a.printf("%s\n", l)
	}
}

// printAccountUsed is printAccountSource for a command that shows no more of
// the account than its id: it adds the plan where the API names the account.
func (a *app) printAccountUsed() {
	x := a.accountDetails()
	a.printAccountSource(x)
	if x == nil {
		return
	}
	switch lines, _ := a.accountFacts(x); {
	case len(lines) == 0:
	case a.planText(x) != "":
		a.printf("Plan         %s\n", a.planText(x))
	case deref(x.Plan.Name) != "":
		a.printf("Plan         %s\n", *x.Plan.Name)
	}
}

// planText is what to print as an account's plan instead of what the API
// names, "" for the API's own.
func (a *app) planText(x *api.Account) string {
	if a.planInstead == nil {
		return ""
	}
	return a.planInstead(x)
}

// warnAccountIgnored says once, on stderr and never with --json, that an API
// key makes ATMEE_ACCOUNT_ID count for nothing: a key has its own account.
func (a *app) warnAccountIgnored() {
	if a.jsonMode || a.warnedAccount || (creds.EnvAccountID() == "" && a.accountFlag == "") {
		return
	}
	a.warnedAccount = true
	if a.accountFlag != "" {
		a.note("Warning: --account is ignored, because an API key is in use and a key acts for its own account.\n")
		return
	}
	if a.store.EnvAPIKey() != "" {
		a.note("Warning: ATMEE_ACCOUNT_ID is ignored, because ATMEE_API_KEY is set and a key acts for its own account.\n")
		return
	}
	a.note("Warning: ATMEE_ACCOUNT_ID is ignored, because an API key is saved and a key acts for its own account.\n")
}

// accountHint is what to do about an account the API refused, "" for any
// other error.
func (a *app) accountHint(err error) string {
	var e *api.Error
	if !errors.As(err, &e) || e.Code != api.CodeAccountNotAllowed {
		return ""
	}
	pick := a.pickAccount
	if pick == "" {
		pick = "`atmee login --account \"\"` goes back to the default account"
	}
	switch source := a.accountSource(); {
	case source == accountFromFlag && a.flagHint != "":
		return a.flagHint
	case source == accountFromEnv:
		return "ATMEE_ACCOUNT_ID takes the id of an account you belong to: fix or unset it. Without it, " + pick
	}
	return pick
}

// userLogin refuses what only a signed-in user may do where nobody is signed
// in: an API key would not help, so it is not offered.
func (a *app) userLogin() error {
	if a.store.Session() == nil {
		return errors.New("not signed in: run `atmee login`")
	}
	return nil
}

// accountProblem says whether a lookup failed for the account it names: one
// that is not the user's, or a name that means none or several.
func accountProblem(err error) bool {
	var u usageError
	var e *api.Error
	return errors.As(err, &u) || (errors.As(err, &e) && e.Code == api.CodeAccountNotAllowed)
}

// ambiguousAccounts are the accounts a 409 account_ambiguous offers, nil for
// any other error.
func ambiguousAccounts(err error) []api.AccountChoice {
	var e *api.Error
	if errors.As(err, &e) && e.Code == api.CodeAccountAmbiguous {
		return e.Accounts
	}
	return nil
}

// noteAccounts lists on stderr the accounts a refusal offers, and how to pick one.
func (a *app) noteAccounts(err error) {
	choices := ambiguousAccounts(err)
	if len(choices) == 0 {
		return
	}
	a.note("Your accounts (pick one with `atmee login --account <id>`, or set ATMEE_ACCOUNT_ID):\n")
	for _, c := range choices {
		if c.Username != "" {
			a.note("  %s  @%s\n", c.ID, c.Username)
		} else {
			a.note("  %s\n", c.ID)
		}
	}
}

// userSession signs requests as the signed-in user: the access token as
// Authorization: Bearer, refreshed and saved again a minute before it
// expires, and X-Account-Id when an account was chosen.
type userSession struct {
	mu    sync.Mutex
	store creds.Store
	s     creds.Session
	hc    *http.Client
	// named is the account this call names, over any other.
	named string
	// noAccount names none, for a call about the user and not an account.
	noAccount bool
	// ignoreEnv sends the session's own account, whatever ATMEE_ACCOUNT_ID says.
	ignoreEnv bool
}

func (u *userSession) Authorize(ctx context.Context, r *http.Request) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if time.Until(u.s.ExpiresAt) < time.Minute {
		// Another atmee may have refreshed (and rotated the refresh token)
		// since this one read the file.
		if latest := u.store.Session(); latest != nil && latest.ExpiresAt.After(u.s.ExpiresAt) {
			u.s = *latest
		}
	}
	if time.Until(u.s.ExpiresAt) < time.Minute {
		if u.s.RefreshToken == "" {
			// Nothing to renew with: use the token for as long as it is valid.
			if !time.Now().Before(u.s.ExpiresAt) {
				return errors.New("the sign-in expired (the server issued no refresh token); run `atmee login` again")
			}
			return u.sign(r)
		}
		t, err := oauth.Refresh(ctx, u.hc, u.s.TokenEndpoint, u.s.ClientID, u.s.RefreshToken, u.s.Resource)
		if err != nil {
			return fmt.Errorf("the sign-in expired and could not be renewed (%w); run `atmee login` again", err)
		}
		u.s.AccessToken, u.s.RefreshToken, u.s.ExpiresAt = t.AccessToken, t.RefreshToken, t.ExpiresAt
		if err := u.store.SaveSession(u.s); err != nil {
			return err
		}
	}
	return u.sign(r)
}

func (u *userSession) sign(r *http.Request) error {
	r.Header.Set("Authorization", "Bearer "+u.s.AccessToken)
	id := accountID(u.s)
	if u.ignoreEnv {
		id = u.s.AccountID
	}
	if u.named != "" {
		id = u.named
	}
	if u.noAccount {
		id = ""
	}
	if id != "" {
		r.Header.Set("X-Account-Id", id)
	}
	return nil
}
