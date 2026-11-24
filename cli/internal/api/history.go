package api

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// The account's generation history: every clip, portrait and voice call that
// finished (charged) or is still running, newest first, with who made it,
// its inputs and its outputs. Reading charges nothing; deleting removes a
// generation's outputs and refunds nothing.
type (
	Generation        = avatarapi.Generation
	GenerationPage    = avatarapi.GenerationListResponse
	GenerationSummary = avatarapi.GenerationSummaryResponse
)

// GenerationFilter narrows the history. Empty fields and nil leave a filter
// out. Kind and Modes apply to the list only; the summary counts every kind.
type GenerationFilter struct {
	// Kind is clip, portrait or voice.
	Kind string
	// Modes are picture, avatar or unknown (clips only).
	Modes []string
	// MadeBy are me, apiKeys, former, a member's user id or an API key id.
	MadeBy []string
	// Statuses are rendering, ready, expiringSoon, expired or deleted.
	Statuses []string
	Since    *time.Time
	// Query matches the prompt, script or description, ignoring case.
	Query string
}

// The values the history's filters take.
var (
	GenerationKinds    = []string{"clip", "portrait", "voice"}
	GenerationModes    = []string{"picture", "avatar", "unknown"}
	GenerationStatuses = []string{"rendering", "ready", "expiringSoon", "expired", "deleted"}
	// GenerationOperations are the operations a delete may name.
	GenerationOperations = []string{"clip", "portrait_generation", "portrait_enhancement", "voice_design"}
)

func optList(v []string) *[]string {
	if len(v) == 0 {
		return nil
	}
	return &v
}

func optStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// Generations returns one page of the history, newest first. cursor is the
// previous page's nextCursor (empty for the first page), with the same
// filter; limit 0 leaves the API's default.
func (c *Client) Generations(ctx context.Context, f GenerationFilter, limit int, cursor string) (*GenerationPage, error) {
	params := &avatarapi.ListGenerationsParams{
		Mode: optList(f.Modes), MadeBy: optList(f.MadeBy), Status: optList(f.Statuses),
		Since: f.Since, Q: optStr(f.Query), Cursor: optStr(cursor),
	}
	if f.Kind != "" {
		k := avatarapi.ListGenerationsParamsKind(f.Kind)
		params.Kind = &k
	}
	if limit > 0 {
		params.Limit = &limit
	}
	r, err := c.gen.ListGenerationsWithResponse(ctx, params)
	if r == nil {
		return nil, err
	}
	return decode[GenerationPage](r.HTTPResponse, r.Body, err)
}

// GenerationSummary counts the history by kind and clip mode under the
// filter (its Kind and Modes are not sent) and lists everyone who made any
// of the account's generations.
func (c *Client) GenerationSummary(ctx context.Context, f GenerationFilter) (*GenerationSummary, error) {
	params := &avatarapi.GetGenerationSummaryParams{
		MadeBy: optList(f.MadeBy), Status: optList(f.Statuses), Since: f.Since, Q: optStr(f.Query),
	}
	r, err := c.gen.GetGenerationSummaryWithResponse(ctx, params)
	if r == nil {
		return nil, err
	}
	return decode[GenerationSummary](r.HTTPResponse, r.Body, err)
}

// ParseGenerationRequestID validates a generation's request id (the
// Idempotency-Key its call was sent with) before any request.
func ParseGenerationRequestID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, fmt.Errorf("%q is not a generation's request id (a UUID; find it with `atmee generate list`)", s)
	}
	return id, nil
}

// DeleteGeneration deletes a generation's outputs: their files go and their
// links stop working; its inputs and poster stay. Deleting again is no
// error. operation (empty to leave it out) is needed only when the request
// id was used for several operations (409 generation_ambiguous).
func (c *Client) DeleteGeneration(ctx context.Context, requestID, operation string) error {
	id, err := ParseGenerationRequestID(requestID)
	if err != nil {
		return err
	}
	params := &avatarapi.DeleteGenerationParams{}
	if operation != "" {
		op := avatarapi.DeleteGenerationParamsOperation(operation)
		params.Operation = &op
	}
	r, err := c.gen.DeleteGenerationWithResponse(ctx, id, params)
	if r == nil {
		return err
	}
	_, err = decode[struct{}](r.HTTPResponse, r.Body, err)
	return err
}
