package api

import (
	"context"

	"github.com/Atmanity/toolkit/cli/internal/avatarapi"
)

// The account reads, as the API answers them (camelCase, microcredits).
type (
	Balance     = avatarapi.AccountBalance
	Account     = avatarapi.Account
	CurrentUser = avatarapi.CurrentCaller
	Redemption  = avatarapi.CodeRedemption
	Earnings    = avatarapi.AccountEarnings
)

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
