package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// Speech sessions: realtime speech-to-speech conversations over a WebSocket,
// without video.
type (
	SpeechSession       = avatarapi.SpeechSessionCreateResponse
	SpeechSessionStatus = avatarapi.SpeechSessionStatusResponse
	SpeechSessionEnd    = avatarapi.SpeechSessionEndResponse
)

// SpeechSessionInput is a speech session start; AvatarID is required.
type SpeechSessionInput struct {
	AvatarID           uuid.UUID
	MaxDurationSeconds *int
	Language           string
	// Settings is the pinned settings object (see ParseSpeechSettings), sent
	// as given so fields the CLI does not model reach the API unchanged.
	Settings json.RawMessage
}

// speechSettingsFields are the pinned settings the API accepts, read off the
// generated type so they follow the spec.
var speechSettingsFields = func() map[string]bool {
	out := map[string]bool{}
	t := reflect.TypeOf(avatarapi.SpeechSessionPinnedSettings{})
	for i := 0; i < t.NumField(); i++ {
		if name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
			out[name] = true
		}
	}
	return out
}()

// ParseSpeechSettings checks a pinned-settings file: a JSON object whose
// fields are the API's (instructions, persona, facts, ...). The API checks
// their limits (422 invalid_settings).
func ParseSpeechSettings(raw []byte) (map[string]json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return nil, fmt.Errorf("the settings file must hold a JSON object of session.start fields, e.g. {\"instructions\": \"...\"}")
	}
	for k := range top {
		if !speechSettingsFields[k] {
			known := make([]string, 0, len(speechSettingsFields))
			for f := range speechSettingsFields {
				known = append(known, f)
			}
			sort.Strings(known)
			return nil, fmt.Errorf("%q is not a setting that can be pinned (one of %s)", k, strings.Join(known, ", "))
		}
	}
	return top, nil
}

// StartSpeechSession reserves a speech session. A transport failure is not
// resent: a reservation nobody connects to costs nothing and closes by
// itself, but a resend would hold a second one against the key's limits.
func (c *Client) StartSpeechSession(ctx context.Context, in SpeechSessionInput) (*SpeechSession, error) {
	body := struct {
		AvatarID           uuid.UUID       `json:"avatarId"`
		MaxDurationSeconds *int            `json:"maxDurationSeconds,omitempty"`
		Language           *string         `json:"language,omitempty"`
		Settings           json.RawMessage `json:"settings,omitempty"`
	}{AvatarID: in.AvatarID, MaxDurationSeconds: in.MaxDurationSeconds, Settings: in.Settings}
	if in.Language != "" {
		body.Language = &in.Language
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.CreateSpeechSessionWithBodyWithResponse(ctx, "application/json", bytes.NewReader(raw))
	if r == nil {
		return nil, sentOnce(err, "a session nobody connects to closes by itself within a few minutes; check your concurrent sessions")
	}
	return decode[SpeechSession](r.HTTPResponse, r.Body, err)
}

// SpeechSessionStatus reads a speech session's state, end reason and cost.
func (c *Client) SpeechSessionStatus(ctx context.Context, sessionID string) (*SpeechSessionStatus, error) {
	id, err := ParseID(sessionID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetSpeechSessionWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[SpeechSessionStatus](r.HTTPResponse, r.Body, err)
}

// EndSpeechSession ends a speech session. With no connection open it
// settles at once; with one open the answer is still active, with
// EndRequestedAt set, and the session settles at its next usage report
// (within 30 seconds), billed until then.
func (c *Client) EndSpeechSession(ctx context.Context, sessionID string) (*SpeechSessionEnd, error) {
	id, err := ParseID(sessionID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.EndSpeechSessionWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[SpeechSessionEnd](r.HTTPResponse, r.Body, err)
}
