package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Atmanity/toolkit/cli/internal/avatarapi"
)

// Generated portraits and voices. Each comes back as upload:<uploadId>, which
// a create or a replace takes where it takes an https URL. A clip comes back
// the same way, with a link to download it.
type (
	GeneratedPortraits = avatarapi.GeneratedPortraitsResponse
	DesignedVoices     = avatarapi.DesignedVoicesResponse
	GeneratedClip      = avatarapi.GeneratedClipResponse
)

// DesignVoiceInput is a voice design; only Description is required.
type DesignVoiceInput struct {
	Description, AvatarName, Language, SampleText string
}

// ClipInput is a clip; Image and Prompt are required, and Duration is required
// with Audio. Zero values and nil are left for the API's defaults.
type ClipInput struct {
	Image, Prompt, Audio string
	Duration             int
	Resolution           *Resolution
	Seed                 *int64
}

const (
	// generationAttempt bounds one request. The server answers
	// request_in_progress at its own soft deadline, well before this.
	generationAttempt = 5 * time.Minute
	// generationBudget bounds the waits for a call that is still running.
	generationBudget = 10 * time.Minute
	// uploadPut bounds sending a portrait to the upload target.
	uploadPut = 5 * time.Minute
	// uploadPolls are the reads of a sent upload until it reads uploaded.
	uploadPolls = 20
)

// generating runs a paid generation call under one Idempotency-Key. A call
// the server still reports running (409 request_in_progress) is sent again
// with the same key after Retry-After, which returns its one result, charged
// once. Nothing is sent again after a network failure or a proxy's 5xx, which
// may follow a charge: the error carries the key for a deliberate retry.
func generating[T any](ctx context.Context, c *Client, key string, call func(ctx context.Context, key string) (*T, error)) (*T, error) {
	if key == "" {
		key = uuid.NewString()
	}
	deadline := c.now().Add(generationBudget)
	for {
		attempt, cancel := context.WithTimeout(ctx, generationAttempt)
		out, err := call(attempt, key)
		cancel()
		if err == nil {
			return out, nil
		}
		var apiErr *Error
		if !errors.As(err, &apiErr) || (apiErr.Status >= 500 && apiErr.Code == "") {
			return nil, fmt.Errorf("%w (it may have reached the server; retry with --idempotency-key %s: the same key never charges twice)", err, key)
		}
		if apiErr.Code != "request_in_progress" {
			return nil, err
		}
		wait := time.Duration(apiErr.RetryAfter) * time.Second
		if wait <= 0 {
			wait = c.InProgressSleep
		}
		if c.now().Add(wait).After(deadline) {
			return nil, fmt.Errorf("%w (still running; retry with --idempotency-key %s to get its one result)", err, key)
		}
		if err := c.sleepCtx(ctx, wait); err != nil {
			return nil, err
		}
	}
}

// GeneratePortraits makes portraits of an invented character from a description.
func (c *Client) GeneratePortraits(ctx context.Context, description, key string) (*GeneratedPortraits, error) {
	return generating(ctx, c, key, func(ctx context.Context, key string) (*GeneratedPortraits, error) {
		r, err := c.gen.GeneratePortraitsWithResponse(ctx, &avatarapi.GeneratePortraitsParams{IdempotencyKey: key},
			avatarapi.GeneratePortraitsRequest{Description: description})
		if r == nil {
			return nil, err
		}
		return decode[GeneratedPortraits](r.HTTPResponse, r.Body, err)
	})
}

// EnhancePortrait enhances a photo, or restyles it when prompt is given.
// image is an https URL or upload:<uploadId>.
func (c *Client) EnhancePortrait(ctx context.Context, image, prompt, key string) (*GeneratedPortraits, error) {
	req := avatarapi.EnhancePortraitRequest{Image: image}
	if prompt != "" {
		req.Prompt = &prompt
	}
	return generating(ctx, c, key, func(ctx context.Context, key string) (*GeneratedPortraits, error) {
		r, err := c.gen.EnhancePortraitWithResponse(ctx, &avatarapi.EnhancePortraitParams{IdempotencyKey: key}, req)
		if r == nil {
			return nil, err
		}
		return decode[GeneratedPortraits](r.HTTPResponse, r.Body, err)
	})
}

// DesignVoice makes voice previews from a description.
func (c *Client) DesignVoice(ctx context.Context, in DesignVoiceInput, key string) (*DesignedVoices, error) {
	req := avatarapi.DesignVoiceRequest{VoiceDescription: in.Description}
	if in.AvatarName != "" {
		req.AvatarName = &in.AvatarName
	}
	if in.Language != "" {
		l := avatarapi.DesignVoiceRequestLanguage(in.Language)
		req.Language = &l
	}
	if in.SampleText != "" {
		req.SampleText = &in.SampleText
	}
	return generating(ctx, c, key, func(ctx context.Context, key string) (*DesignedVoices, error) {
		r, err := c.gen.DesignVoiceWithResponse(ctx, &avatarapi.DesignVoiceParams{IdempotencyKey: key}, req)
		if r == nil {
			return nil, err
		}
		return decode[DesignedVoices](r.HTTPResponse, r.Body, err)
	})
}

// GenerateClip renders a clip from an image and a prompt.
func (c *Client) GenerateClip(ctx context.Context, in ClipInput, key string) (*GeneratedClip, error) {
	req := avatarapi.GenerateClipRequest{Image: in.Image, Prompt: in.Prompt, Resolution: in.Resolution, Seed: in.Seed}
	if in.Audio != "" {
		req.Audio = &in.Audio
	}
	if in.Duration > 0 {
		req.DurationSeconds = &in.Duration
	}
	return generating(ctx, c, key, func(ctx context.Context, key string) (*GeneratedClip, error) {
		r, err := c.gen.GenerateClipWithResponse(ctx, &avatarapi.GenerateClipParams{IdempotencyKey: key}, req)
		if r == nil {
			return nil, err
		}
		return decode[GeneratedClip](r.HTTPResponse, r.Body, err)
	})
}

// The types an upload takes, by file extension.
var (
	portraitTypes = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}
	voiceTypes    = map[string]string{".mp3": "audio/mpeg", ".wav": "audio/wav", ".m4a": "audio/mp4", ".aac": "audio/aac",
		".ogg": "audio/ogg", ".flac": "audio/flac", ".opus": "audio/opus", ".webm": "audio/webm"}
)

// UploadPortrait sends a local image through the upload API and returns its
// upload:<uploadId>, for the calls that take no file in their own request.
func (c *Client) UploadPortrait(ctx context.Context, path string) (string, error) {
	return c.uploadFile(ctx, path, avatarapi.UploadKindPortrait, portraitTypes, "a portrait", "a jpg, png or webp")
}

// UploadVoice is UploadPortrait for a local audio file.
func (c *Client) UploadVoice(ctx context.Context, path string) (string, error) {
	return c.uploadFile(ctx, path, avatarapi.UploadKindVoice, voiceTypes, "audio", "an mp3, wav, m4a, aac, ogg, flac, opus or webm")
}

// uploadFile sends a local file of one of types through the upload API; what
// and formats name it in the refusals.
func (c *Client) uploadFile(ctx context.Context, path string, kind avatarapi.UploadKind, types map[string]string, what, formats string) (string, error) {
	info, err := os.Stat(expandHome(path))
	if err != nil {
		return "", fmt.Errorf("%s: file not found", path)
	}
	contentType, ok := types[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", fmt.Errorf("%s: %s is %s", path, what, formats)
	}
	name := filepath.Base(path)
	r, err := c.gen.CreateUploadWithResponse(ctx, avatarapi.UploadCreateRequest{
		Kind: kind, FileName: &name, ContentType: &contentType,
	})
	if r == nil {
		return "", err
	}
	u, err := decode[avatarapi.UploadCreateResponse](r.HTTPResponse, r.Body, err)
	if err != nil {
		return "", err
	}
	if info.Size() > u.MaxBytes {
		return "", fmt.Errorf("%s is over the %d MB limit for %s", path, u.MaxBytes/(1<<20), what)
	}
	data, err := os.ReadFile(expandHome(path))
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if err := putUpload(ctx, u, contentType, data); err != nil {
		return "", err
	}
	for i := 0; i < uploadPolls; i++ {
		s, err := c.gen.GetUploadWithResponse(ctx, u.UploadId)
		if s == nil {
			return "", err
		}
		got, err := decode[avatarapi.Upload](s.HTTPResponse, s.Body, err)
		if err != nil {
			return "", err
		}
		if got.Status == avatarapi.UploadStatusUploaded {
			return "upload:" + u.UploadId.String(), nil
		}
		if err := c.sleepCtx(ctx, 500*time.Millisecond); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("upload %s did not arrive", u.UploadId)
}

// putUpload sends the bytes to the signed target, which takes no credential.
func putUpload(ctx context.Context, u *avatarapi.UploadCreateResponse, contentType string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, uploadPut)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.UploadUrl, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range u.UploadHeaders {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return check(resp, body)
}

// replaceFromRef is the JSON form of a replace: field names an https URL or
// upload:<uploadId>.
func replaceFromRef(field, ref string) io.Reader {
	b, _ := json.Marshal(map[string]string{field: ref})
	return bytes.NewReader(b)
}
