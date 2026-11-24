package api

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// The account reads, as the API answers them (camelCase, microcredits).
type (
	Balance     = avatarapi.AccountBalance
	CurrentUser = avatarapi.CurrentCaller
	Earnings    = avatarapi.AccountEarnings
)

// Account is the plan, limits, prices and capture quota, as the API
// answers it.
type Account struct {
	avatarapi.Account
	// SessionV2Reported is whether pricing carried
	// sessionV2PerMinuteMicrocredits: a server older than the v2 rates leaves
	// it out, and its one conversation rate covers both engines. It tells
	// that apart from a reported rate of 0.
	SessionV2Reported bool `json:"-"`
	// V2Trial is limits.v2Trial: the account's window for trying Atmee V2,
	// where the server reports one. Absent (nil) on servers and accounts
	// without it.
	V2Trial *V2Trial `json:"-"`
	// rawLimits are limits.v2Trial and limits.v2Unlock (Limits.V2Unlock) as
	// the server sent them, given back as sent by --json (a null stays null,
	// and a field the server left out stays out).
	rawLimits map[string]json.RawMessage
	// Extra are the fields the server sent that the generated Account has
	// not. --json leaves them out.
	Extra map[string]json.RawMessage `json:"-"`
	// missing are the v2 rates the server left out, kept out of the JSON
	// again rather than written as 0.
	missing []string
}

// V2Trial is a window in which the account may try Atmee V2: its state
// (none, not_started, active or ended), when it starts and ends, and how
// many days a trial runs. Days is nil on a server older than the field; 0
// means no trial is offered.
type V2Trial struct {
	State    string     `json:"state"`
	Days     *int       `json:"days,omitempty"`
	StartsAt *time.Time `json:"startsAt,omitempty"`
	EndsAt   *time.Time `json:"endsAt,omitempty"`
}

// passLimits are the limits fields given back to --json as the server sent
// them: the generated limits write a days the server never sent as 0, and a
// v2Unlock it left out as null.
var passLimits = []string{"v2Trial", "v2Unlock"}

// v2Rates are the pricing fields an older server leaves out. The V2 HD
// rates (sessionV2HdPerMinuteMicrocredits and the like) are optional in
// the spec, so the generated pointers already tell absent (nil) from a
// reported 0, and their JSON leaves them out when absent.
var v2Rates = []string{"sessionV2PerMinuteMicrocredits", "monetizedV2PerMinuteMicrocredits"}

func (a *Account) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &a.Account); err != nil {
		return err
	}
	var p struct {
		Pricing map[string]json.RawMessage `json:"pricing"`
		Limits  map[string]json.RawMessage `json:"limits"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	a.V2Trial, a.rawLimits = nil, map[string]json.RawMessage{}
	for _, f := range passLimits {
		if v, ok := p.Limits[f]; ok {
			a.rawLimits[f] = v
		}
	}
	if v, ok := a.rawLimits["v2Trial"]; ok {
		if err := json.Unmarshal(v, &a.V2Trial); err != nil {
			return err
		}
	}
	a.missing = nil
	for _, f := range v2Rates {
		if _, ok := p.Pricing[f]; !ok {
			a.missing = append(a.missing, f)
		}
	}
	_, a.SessionV2Reported = p.Pricing["sessionV2PerMinuteMicrocredits"]
	a.Extra = nil
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return err
	}
	for k, v := range all {
		if !accountFields[k] {
			if a.Extra == nil {
				a.Extra = map[string]json.RawMessage{}
			}
			a.Extra[k] = v
		}
	}
	return nil
}

// accountFields are the JSON names of the generated Account.
var accountFields = func() map[string]bool {
	names := map[string]bool{}
	t := reflect.TypeFor[avatarapi.Account]()
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		names[name] = true
	}
	return names
}()

func (a Account) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(a.Account)
	if err != nil {
		return b, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if len(a.missing) > 0 {
		var pricing map[string]json.RawMessage
		if err := json.Unmarshal(m["pricing"], &pricing); err != nil {
			return nil, err
		}
		for _, f := range a.missing {
			delete(pricing, f)
		}
		if m["pricing"], err = json.Marshal(pricing); err != nil {
			return nil, err
		}
	}
	// limits.v2Trial and limits.v2Unlock go back as the server sent them.
	var limits map[string]json.RawMessage
	if err := json.Unmarshal(m["limits"], &limits); err != nil {
		return nil, err
	}
	for _, f := range passLimits {
		delete(limits, f)
		if v, ok := a.rawLimits[f]; ok {
			limits[f] = v
		}
	}
	if m["limits"], err = json.Marshal(limits); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Redemption is what a redeemed code gave the account, as the API answers
// it.
type Redemption struct {
	avatarapi.CodeRedemption
	// unlockSent is whether the server sent v2Unlock (null included): an
	// older server leaves it out, and --json keeps it out rather than
	// writing the generated field's null.
	unlockSent bool
}

func (r *Redemption) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &r.CodeRedemption); err != nil {
		return err
	}
	var p map[string]json.RawMessage
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	_, r.unlockSent = p["v2Unlock"]
	return nil
}

func (r Redemption) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(r.CodeRedemption)
	if err != nil || r.unlockSent {
		return b, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	delete(m, "v2Unlock")
	return json.Marshal(m)
}

// Balance is the account's credits: what a session can spend now, the
// buckets, what live sessions hold and what expires when.
func (c *Client) Balance(ctx context.Context) (*Balance, error) {
	r, err := c.gen.GetAccountBalanceWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[Balance](r.HTTPResponse, r.Body, err)
}

// Account is the plan, limits, prices and capture quota.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	r, err := c.gen.GetAccountWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[Account](r.HTTPResponse, r.Body, err)
}

// CurrentKey is the key making the call: name, scopes, spend cap.
func (c *Client) CurrentKey(ctx context.Context) (*CurrentUser, error) {
	r, err := c.gen.GetCurrentApiKeyWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[CurrentUser](r.HTTPResponse, r.Body, err)
}

// Redeem redeems a code for the account: credits, or access to Atmee V2 or
// V2 HD. Only a signed-in user may; an API key gets 403 login_required.
func (c *Client) Redeem(ctx context.Context, code string) (*Redemption, error) {
	r, err := c.gen.RedeemAccountCodeWithResponse(ctx, avatarapi.RedeemCodeRequest{Code: code})
	if r == nil {
		return nil, err
	}
	return decode[Redemption](r.HTTPResponse, r.Body, err)
}

// Earnings is the creator wallet: what monetized avatars earned, what was
// paid out, and the balance between them.
func (c *Client) Earnings(ctx context.Context) (*Earnings, error) {
	r, err := c.gen.GetAccountEarningsWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[Earnings](r.HTTPResponse, r.Body, err)
}
