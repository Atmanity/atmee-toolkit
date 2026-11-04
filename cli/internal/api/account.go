package api

import (
	"context"
	"encoding/json"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// The account reads, as the API answers them (camelCase, microcredits).
type (
	Balance     = avatarapi.AccountBalance
	CurrentUser = avatarapi.CurrentCaller
	Redemption  = avatarapi.CodeRedemption
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
	// missing are the v2 rates the server left out, kept out of the JSON
	// again rather than written as 0.
	missing []string
}

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
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	a.missing = nil
	for _, f := range v2Rates {
		if _, ok := p.Pricing[f]; !ok {
			a.missing = append(a.missing, f)
		}
	}
	_, a.SessionV2Reported = p.Pricing["sessionV2PerMinuteMicrocredits"]
	return nil
}

func (a Account) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(a.Account)
	if err != nil || len(a.missing) == 0 {
		return b, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
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

// Redeem redeems an invite or credit code for the account: credits, v2
// access, or both. Only a signed-in user may; an API key gets 403
// login_required.
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
