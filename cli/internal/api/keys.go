package api

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// API keys, managed by a signed-in user (an API key is answered 403
// login_required): the account's keys, and a new key or a new secret, which
// the API returns only once.
type (
	Key       = avatarapi.ApiKey
	KeySecret = avatarapi.ApiKeySecret
)

// ParseKeyID validates a key id before any request.
func ParseKeyID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, fmt.Errorf("%q is not an API key id (a UUID; find it with `atmee keys list`)", s)
	}
	return id, nil
}

// APIKeys lists the account's keys, newest first, revoked ones included.
func (c *Client) APIKeys(ctx context.Context) ([]Key, error) {
	r, err := c.gen.ListApiKeysWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	out, err := decode[avatarapi.ApiKeyList](r.HTTPResponse, r.Body, err)
	if err != nil {
		return nil, err
	}
	return out.ApiKeys, nil
}

// CreateAPIKey makes a key; its secret is in the answer, and only there.
func (c *Client) CreateAPIKey(ctx context.Context, fields map[string]any) (*KeySecret, error) {
	r, err := c.gen.CreateApiKeyWithBodyWithResponse(ctx, "application/json", jsonBody(fields))
	if r == nil {
		// The key may exist with a secret nobody saw: look before making another.
		return nil, sentOnce(err, "check `atmee keys list` (and revoke a key you cannot use)")
	}
	return decode[KeySecret](r.HTTPResponse, r.Body, err)
}

// UpdateAPIKey changes the fields given (API names; a nil avatarId widens the
// key to the whole account) and answers the key as saved, whose limits may be
// below what was sent: the plan's ceilings apply on every edit. A failed edit
// is safe to send again.
func (c *Client) UpdateAPIKey(ctx context.Context, keyID string, fields map[string]any) (*Key, error) {
	id, err := ParseKeyID(keyID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.UpdateApiKeyWithBodyWithResponse(ctx, id, "application/json", jsonBody(fields))
	if r == nil {
		return nil, err
	}
	return decode[Key](r.HTTPResponse, r.Body, err)
}

// RotateAPIKey issues a new secret for the key; the old one stops working.
func (c *Client) RotateAPIKey(ctx context.Context, keyID string) (*KeySecret, error) {
	id, err := ParseKeyID(keyID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.RotateApiKeyWithResponse(ctx, id)
	if r == nil {
		// The old secret may already be dead: rotate again to get a new one.
		return nil, sentOnce(err, "the old secret may have stopped working; rotate again")
	}
	return decode[KeySecret](r.HTTPResponse, r.Body, err)
}

// RevokeAPIKey revokes the key for good (again is no error).
func (c *Client) RevokeAPIKey(ctx context.Context, keyID string) (*Key, error) {
	id, err := ParseKeyID(keyID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.RevokeApiKeyWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Key](r.HTTPResponse, r.Body, err)
}
