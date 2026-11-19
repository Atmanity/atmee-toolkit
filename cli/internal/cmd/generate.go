package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// Generation answers with upload:<uploadId> references, which create and the
// replace commands take as they are.

// portraitsNote replaces generationNote's first sentence for portraits:
// one flat price per call, whether one portrait comes back or two.
const portraitsNote = "Costs the portrait price (`atmee account`) once per call, charged when at least one portrait comes back, so one " +
	"portrait costs the same as two; a flagged, failed or refused call costs nothing. A paid plan's free generation allowance pays " +
	"first. Results (upload:<id>) last 7 days and work once; their printed links expire sooner. A signed-in user, or an API key with generation:write and avatars:write."

const generationNote = "Costs the operation's price (`atmee account`), charged only when something comes back; a flagged, failed or refused " +
	"call costs nothing. Results (upload:<id>) last 7 days and work once; their printed links expire sooner. A signed-in user, or an API key with generation:write and avatars:write."

const (
	enhanceEngineHelp = "The engine the avatar will be created with, v2hd (Atmee V2 HD, where the server offers it), v2 or v1: " +
		"v1 turns the face to the camera, shoulders up, v2 and v2hd keep the pose, framing and background (the API's default is v1)"
	portraitsEngineHelp = "The engine the avatar will be created with, v2hd (Atmee V2 HD, where the server offers it), v2 or v1: " +
		"v1 frames the character from the shoulders up, v2 and v2hd half-body unless the description sets another framing " +
		"(the API's default is v1)"
)

// checkPortraitEngine refuses an engine the API does not know before any
// request; both generation requests take the same engines.
func checkPortraitEngine(engine string) error {
	if engine != "" && !avatarapi.EnhancePortraitRequestEngine(engine).Valid() {
		return usagef("--engine takes v2hd, v2 or v1 (%s)", engine)
	}
	return nil
}

const idempotencyHelp = "Reuse the key a failed call printed to retry it: the same key never charges twice and returns the first result"

func (a *app) generateCmd() *cobra.Command {
	return group("generate", "Make portraits and voices for an avatar, or a video clip (portraits, enhance, voice, clip, clip-status)",
		a.generatePortraitsCmd(), a.generateEnhanceCmd(), a.generateVoiceCmd(), a.generateClipCmd(), a.generateClipStatusCmd())
}

func (a *app) generatePortraitsCmd() *cobra.Command {
	var description, engine, key string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "portraits --description TEXT",
		Short: "Generate portraits of an invented character from a description.",
		Long: "Generate up to two square portraits of an invented character, one from Gemini and one from Grok. Never use it to depict a " +
			"real person from a description: for a real person, with their permission, use `atmee generate enhance` on their own photo. " + portraitsNote,
		Example: `  atmee generate portraits --description "a weathered lighthouse keeper, sixties, warm smile"`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(description) == "" || utf8.RuneCountInString(description) > 2000 {
				return usagef("--description takes 1 to 2000 characters: what the portrait shows")
			}
			if err := checkPortraitEngine(engine); err != nil {
				return err
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.GeneratePortraits(ctx(), description, engine, key)
			if err != nil {
				return err
			}
			return a.printPortraits(out, jsonOut)
		},
	}
	f := c.Flags()
	f.StringVar(&description, "description", "", "What the portrait shows, an invented character (required)")
	f.StringVar(&engine, "engine", "", portraitsEngineHelp)
	f.StringVar(&key, "idempotency-key", "", idempotencyHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) generateEnhanceCmd() *cobra.Command {
	var image, prompt, engine, avatar, key string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "enhance --image PATH",
		Short: "Enhance a photo, or restyle it with --prompt.",
		Long: "Crop a photo to a square and return up to two portraits, one from Gemini and one from Grok. Without --prompt they are enhanced " +
			"versions that keep the subject's identity; with it they restyle the photo as asked, keeping the person unless the prompt " +
			"says otherwise. If the photo shows a real person other than you, have their permission first. " + generationNote,
		Example: `  atmee generate enhance --image ./me.jpg
  atmee generate enhance --image ./me.jpg --prompt "as a 1920s detective, film noir lighting"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if image == "" {
				return usagef("--image is required: a local jpg, png or webp, an https URL or upload:ID")
			}
			if strings.HasPrefix(image, "http://") {
				return usagef("--image: asset URLs must be https:// (%s)", image)
			}
			if utf8.RuneCountInString(prompt) > 2000 {
				return usagef("--prompt takes at most 2000 characters")
			}
			if err := checkPortraitEngine(engine); err != nil {
				return err
			}
			in := api.EnhanceInput{Prompt: prompt, Engine: engine}
			if avatar != "" {
				id, err := uuid.Parse(avatar)
				if err != nil {
					return usagef("--avatar: %q is not an avatar id (find it with `atmee avatars list`)", avatar)
				}
				in.AvatarID = &id
			}
			if key != "" && !api.IsRemote(image) {
				return usagef("--idempotency-key retries the same inputs: give --image the upload:ID the failed call printed, not the file again")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			ref := image
			if !api.IsRemote(image) {
				if ref, err = cl.UploadPortrait(ctx(), image); err != nil {
					return err
				}
			}
			in.Image = ref
			out, err := cl.EnhancePortrait(ctx(), in, key)
			if err != nil {
				// A retry must send the same inputs, and a new upload is a new
				// reference: hand this one back with the key.
				if ref != image && strings.Contains(err.Error(), "--idempotency-key") {
					return fmt.Errorf("%w, and --image %s", err, ref)
				}
				return err
			}
			return a.printPortraits(out, jsonOut)
		},
	}
	f := c.Flags()
	f.StringVar(&image, "image", "", "The photo: a local jpg, png or webp (up to 10 MB), an https URL or upload:ID (required)")
	f.StringVar(&prompt, "prompt", "", "How to restyle the photo; without it the photo is only enhanced")
	f.StringVar(&engine, "engine", "", enhanceEngineHelp)
	f.StringVar(&avatar, "avatar", "", "An existing avatar the photo is for, to replace its photo; its own engine is used, so leave out --engine")
	f.StringVar(&key, "idempotency-key", "", idempotencyHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) generateVoiceCmd() *cobra.Command {
	var in api.DesignVoiceInput
	var key string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "voice --description TEXT",
		Short: "Design voice previews from a description.",
		Long: "Design voice previews of 10 to 60 seconds, the length a v2 avatar's voice sample needs. --text is what every preview says, " +
			"in --language: 220-600 characters for Latin and Cyrillic scripts, 120-300 for Chinese, Japanese and Korean, 180-450 for " +
			"Hindi. Without it, Atmee writes a greeting from the description. " + generationNote,
		Example: `  atmee generate voice --description "a calm, warm alto, unhurried, faint Irish accent" --name Ada
  atmee generate voice --description "un conteur grave et posé, la cinquantaine" --language fr`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if n := utf8.RuneCountInString(in.Description); n < 20 || n > 1000 {
				return usagef("--description takes 20 to 1000 characters: age, tone, pace, accent, character")
			}
			if in.Language != "" && !avatarapi.DesignVoiceRequestLanguage(in.Language).Valid() {
				return usagef("--language takes en, zh-TW, zh-CN, ja, ko, fr, vi, es, de, hi or ru")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.DesignVoice(ctx(), in, key)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(out)
			}
			a.printf("%s in %s (%s%s)\n", plural(len(out.Voices), "voice preview"), out.Language,
				paidNote(out.MicrocreditsCharged, out.MicrocreditsFree), replayNote(out.Replayed))
			first := ""
			for _, v := range out.Voices {
				if v.Unavailable {
					a.printf("  %s  %s  used or expired\n", v.Upload, v.Provider)
					continue
				}
				if first == "" {
					first = v.Upload
				}
				a.printf("  %s  %s  %s\n", v.Upload, v.Provider, previewLength(v.DurationSeconds))
				if v.Url != nil {
					a.printf("    %s\n", *v.Url)
				}
			}
			a.noteFailed(out.FailedProviders)
			if first != "" {
				a.printf("Use one: atmee avatars create --name NAME --image IMAGE --voice %s\n  or: atmee avatars replace-voice AVATAR_ID %s\n", first, first)
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&in.Description, "description", "", "How the voice should sound: age, tone, pace, accent, character, 20-1000 characters (required)")
	f.StringVar(&in.AvatarName, "name", "", "The avatar's name; the written sample says it")
	f.StringVar(&in.Language, "language", "", "The language it speaks: en (default), zh-TW, zh-CN, ja, ko, fr, vi, es, de, hi or ru")
	f.StringVar(&in.SampleText, "text", "", "What every preview says; without it, Atmee writes it")
	f.StringVar(&key, "idempotency-key", "", idempotencyHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) generateClipCmd() *cobra.Command {
	var in api.ClipInput
	var avatar, resolution, key string
	var seed int64
	var jsonOut bool
	c := &cobra.Command{
		Use:   "clip (--image PATH --prompt TEXT | --avatar ID --transcript TEXT)",
		Short: "Render a video clip from an image and a prompt, or of a saved avatar speaking a transcript.",
		Long: "Render an mp4 in one of two ways.\n\n" +
			"With --avatar and --transcript (they go together), a saved avatar speaks: its saved picture is the first frame, " +
			"its Fish Audio voice speaks the transcript (up to 1000 characters) and the lips follow. V1 and V2 avatars both work, " +
			"and the look and voice stay the same from clip to clip. --prompt is optional here (by default the avatar speaks to " +
			"camera); --image, --audio and --duration are refused. The clip runs as long as the speech plus 0.4 seconds of silence " +
			"before and after, rounded up to whole seconds, at most 15: speech past 14.2 seconds is cut, so keep each transcript to " +
			"about 14 seconds of speech and make a longer video from several clips. It is charged at the tier its actual length " +
			"falls in, but the balance must cover the size's 15-second price before the speech is made, so it costs from the " +
			"size's up-to-5-seconds price to its 15-second price. An avatar with no Fish Audio voice is refused at no charge. " +
			"An API key needs avatars:read too.\n\n" +
			"With --image and --prompt, the clip starts on the image (any subject: a person, character, animal, product or scene) and moves as the prompt " +
			"says, silent or with --audio. No avatar is made or needed. --duration is required with --audio (it is not read from the file). " +
			"Pass --duration for a silent clip too, any whole number of seconds up to the limit: without it the clip runs 8 seconds. " +
			"Prices come in tiers of up to 5, 10 and 15 seconds, and a clip pays the tier its length falls in.\n\n" +
			"A clip lasts at least 1 second, up to the server's limit (15 seconds on current servers); a longer --duration " +
			"is refused with the limit.\n\n" +
			"--resolution is the clip's size, one the server serves: `atmee account` lists this server's sizes and, on a " +
			"current server, each one's price per clip for up to 5, 10 and 15 seconds, charged at the next length up, so a 7-second " +
			"clip pays the up-to-10-seconds price (for example 480x480, the default there, " +
			"or 1280x720). Without --resolution the server's default size is used. A size it doesn't serve is refused " +
			"with its list.\n\n" +
			"If the image shows a real person other than you, have their permission first. The price is the same with or without audio, " +
			"charged only when the clip comes back; a flagged, failed or refused call costs nothing. " +
			"The result lasts 7 days (its download link sooner; run clip-status for a fresh one). Clips are on every plan, Free included, and need no Atmee V2 access: " +
			"they spend credits at each size's price per clip (atmee account lists them). " +
			"A signed-in user, or an API key with generation:write (an --image clip needs no avatar scope; --image and --audio files upload " +
			"with it too), plus avatars:read for an --avatar clip.",
		Example: `  atmee generate clip --avatar 11111111-2222-4333-8444-555555555555 --transcript "Welcome back. Today: three tips for better sleep."
  atmee generate clip --avatar 11111111-2222-4333-8444-555555555555 --transcript "See you tomorrow." --prompt "she smiles and waves" --resolution 720x1280
  atmee generate clip --image ./me.jpg --prompt "she walks along the beach at sunset"
  atmee generate clip --image upload:ID --prompt "a slow nod" --audio ./line.wav --duration 6 --resolution 480x640
  atmee generate clip --image ./me.jpg --prompt "he waves at the camera" --resolution 1280x720 --duration 10`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := cmd.Flags()
			fromAvatar := f.Changed("avatar") || f.Changed("transcript")
			if err := checkClipMode(f.Changed, fromAvatar, avatar, &in); err != nil {
				return err
			}
			if f.Changed("resolution") {
				r, err := api.ParseClipResolution(resolution)
				if err != nil {
					return usageError{err}
				}
				in.Resolution = &r
			}
			if f.Changed("seed") {
				if seed < 0 {
					return usagef("--seed takes 0 or more")
				}
				in.Seed = &seed
			}
			local := !fromAvatar && (!api.IsRemote(in.Image) || in.Audio != "" && !api.IsRemote(in.Audio))
			if key != "" && local {
				return usagef("--idempotency-key retries the same inputs: give --image and --audio the upload:ID the failed call printed, not the file again")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			var sent []string
			if !fromAvatar && !api.IsRemote(in.Image) {
				if in.Image, err = cl.UploadImage(ctx(), in.Image); err != nil {
					return err
				}
				sent = append(sent, "--image "+in.Image)
			}
			if in.Audio != "" && !api.IsRemote(in.Audio) {
				if in.Audio, err = cl.UploadVoice(ctx(), in.Audio); err != nil {
					return err
				}
				sent = append(sent, "--audio "+in.Audio)
			}
			out, err := cl.GenerateClip(ctx(), in, key)
			if err != nil {
				// A retry must send the same inputs, and a new upload is a new
				// reference: hand them back with the key.
				if len(sent) > 0 && strings.Contains(err.Error(), "--idempotency-key") {
					return fmt.Errorf("%w, and %s", err, strings.Join(sent, " "))
				}
				var apiErr *api.Error
				if errors.As(err, &apiErr) && apiErr.Code == "video_generation_not_in_plan" {
					return fmt.Errorf("%w; video clips are turned off for this account: contact us through Creator Studio's help center", err)
				}
				// Clips are on every plan: a refusal for Atmee V2 access
				// (an older server) gets no upgrade hint, so the
				// error is kept as text, out of engineHint's reach.
				if v2Refused(err) {
					// An older server may word it around invite codes: keep
					// only its status and code.
					msg := err.Error()
					if strings.Contains(strings.ToLower(msg), "invite") {
						if i := strings.Index(msg, ": "); i >= 0 {
							msg = msg[:i]
						}
					}
					return fmt.Errorf("%s; video clips are on every plan and need no Atmee V2 access: contact us through Creator Studio's help center", msg)
				}
				return err
			}
			if jsonOut {
				return a.printJSON(out)
			}
			note := fmt.Sprintf(" (%s credits charged%s)", credits(int64(out.MicrocreditsCharged)), replayNote(out.Replayed))
			if out.Replayed && key == "" {
				// The key is this command's own, so the replay is of its own
				// first attempt: that attempt paid for the clip, once, while
				// the server's answer reports only the replay's charge (0).
				note = ownReplayNote(cl, out.Clip)
			}
			a.printClip(out.Clip, note)
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&avatar, "avatar", "", "A saved avatar to speak --transcript: its picture is the first frame and its Fish Audio voice speaks (goes with --transcript; leave out --image, --audio and --duration)")
	f.StringVar(&in.Transcript, "transcript", "", "What the --avatar says, 1 to 1000 characters, about 14 seconds of speech at most: speech past 14.2 seconds is cut")
	f.StringVar(&in.Image, "image", "", "Who or what the clip shows: a local jpg, png or webp (up to 10 MB), an https URL or upload:ID (required without --avatar)")
	f.StringVar(&in.Prompt, "prompt", "", "The shot and the motion (required with --image; optional with --avatar, which speaks to camera without it): the model reads up to 1,024 tokens, about 5,000 characters of English and about 1,400 of Chinese; the server refuses a longer one")
	f.StringVar(&in.Audio, "audio", "", "What the person says: a speech recording (local file, https URL or upload:ID), its words and voice kept and lip-synced; without it the clip is silent. The clip adds no speech, music or sound effects of its own")
	f.IntVar(&in.Duration, "duration", 0, "Length in seconds, from 1 up to the server's limit (15 on current servers), any whole number of seconds, priced in tiers of up to 5, 10 or 15 seconds, charged at the next length up: required with --audio, set to the audio's length; without it a silent clip runs 8 seconds")
	f.StringVar(&resolution, "resolution", "", "The clip's size as `WxH`, one this server serves (`atmee account` lists them; e.g. 480x480, the default on current servers)")
	f.Int64Var(&seed, "seed", 0, "The render's seed; another seed gives other motion")
	f.StringVar(&key, "idempotency-key", "", idempotencyHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// checkClipMode checks a clip's flags before any request, for the mode they
// choose: --avatar with --transcript (fromAvatar), or --image with --prompt.
// changed reports whether a flag was given.
func checkClipMode(changed func(string) bool, fromAvatar bool, avatar string, in *api.ClipInput) error {
	if utf8.RuneCountInString(in.Prompt) > 8000 {
		return usagef("--prompt takes 1 to 8000 characters: the shot and the motion")
	}
	if fromAvatar {
		for _, flag := range []string{"image", "audio", "duration"} {
			if changed(flag) {
				return usagef("--%s is not taken with --avatar: the avatar's saved picture is the first frame, its voice speaks "+
					"--transcript, and the clip runs as long as the speech", flag)
			}
		}
		if avatar == "" || !changed("transcript") {
			return usagef("--avatar and --transcript go together: the avatar's id and the words it says")
		}
		if strings.TrimSpace(in.Transcript) == "" || utf8.RuneCountInString(in.Transcript) > 1000 {
			return usagef("--transcript takes 1 to 1000 characters: the words the avatar says")
		}
		if changed("prompt") && strings.TrimSpace(in.Prompt) == "" {
			return usagef("--prompt takes 1 to 8000 characters: the shot and the motion (leave it out and the avatar speaks to camera)")
		}
		id, err := uuid.Parse(avatar)
		if err != nil {
			return usagef("--avatar: %q is not an avatar id (find it with `atmee avatars list`)", avatar)
		}
		in.AvatarID = &id
		return nil
	}
	if in.Image == "" {
		return usagef("--image is required: a local jpg, png or webp, an https URL or upload:ID (or make an avatar speak with --avatar and --transcript)")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return usagef("--prompt takes 1 to 8000 characters: the shot and the motion")
	}
	for flag, v := range map[string]string{"--image": in.Image, "--audio": in.Audio} {
		if strings.HasPrefix(v, "http://") {
			return usagef("%s: asset URLs must be https:// (%s)", flag, v)
		}
	}
	if in.Audio != "" && !changed("duration") {
		return usagef("--duration is required with --audio: the audio's length in seconds")
	}
	if changed("duration") && in.Duration < 1 {
		return usagef("--duration takes 1 second or more (the server sets the most; 15 on current servers)")
	}
	return nil
}

// printClip prints a clip: its length and size (then note), its upload
// and how long it lasts, and its download link.
func (a *app) printClip(clip api.GeneratedClipBody, note string) {
	a.printf("Clip of %s at %dx%d%s\n", seconds(clip.DurationSeconds), clip.Width, clip.Height, note)
	if clip.Unavailable {
		a.printf("  %s  expired\n", clip.Upload)
		return
	}
	a.printf("  %s  until %s\n", clip.Upload, stamp(clip.ExpiresAt))
	if clip.Url != nil {
		a.printf("    %s\n", *clip.Url)
	}
}

// clipWait bounds clip-status --wait: as long as generate clip itself waits.
const clipWait = 10 * time.Minute

func (a *app) generateClipStatusCmd() *cobra.Command {
	var wait, jsonOut bool
	c := &cobra.Command{
		Use:   "clip-status REQUEST_ID",
		Short: "Check a video clip by its request id, and get its download link.",
		Long: "Reads the clip a `generate clip` call made or is making under REQUEST_ID: the Idempotency-Key a `generate clip` printed when it stopped waiting, " +
			"or the --idempotency-key you gave it. It charges nothing and never starts a render, so it is safe from another " +
			"shell or machine with the same account.\n\n" +
			"rendering: still running; --wait keeps reading until it is done (up to 10 minutes). " +
			"ready: the clip, with a download link signed again on each read, until its upload expires 7 days after it was made. " +
			"expired: the clip was made but its upload has ended; it is not made again. " +
			"stalled: the call stopped without finishing; run `atmee generate clip` again with the same inputs and " +
			"--idempotency-key REQUEST_ID to resume it, charged at most once (exits non-zero). " +
			"A request id with no clip (never used, or a call that ended without one and charged nothing) is clip_not_found.",
		Example: `  atmee generate clip-status 5b0c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f
  atmee generate clip-status 5b0c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f --wait --json`,
		Args: exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := api.ParseClipRequestID(args[0]); err != nil {
				return usageError{err}
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			var st *api.ClipStatus
			if wait {
				st, err = cl.WaitClip(ctx(), args[0], clipWait, 5*time.Second, func(*api.ClipStatus) {
					if !jsonOut {
						a.note("Rendering...\n")
					}
				})
			} else {
				st, err = cl.GetClip(ctx(), args[0])
			}
			if err != nil {
				return err
			}
			if jsonOut {
				if err := a.printJSON(st); err != nil {
					return err
				}
			}
			switch st.Status {
			case api.ClipStalled:
				return fmt.Errorf("the clip call stopped without finishing; resume it with the same inputs and --idempotency-key %s "+
					"(atmee generate clip ... --idempotency-key %s): it is charged at most once", args[0], args[0])
			case api.ClipRendering:
				if !jsonOut {
					after := ""
					if st.RetryAfterSeconds != nil {
						after = fmt.Sprintf("; check again in %d s", *st.RetryAfterSeconds)
					}
					a.printf("Rendering%s (or add --wait)\n", after)
				}
			default:
				if !jsonOut && st.Clip != nil {
					a.printClip(*st.Clip, "")
				}
			}
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&wait, "wait", false, "Keep reading until the clip is ready, expired or stalled (up to 10 minutes)")
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) printPortraits(out *api.GeneratedPortraits, jsonOut bool) error {
	if jsonOut {
		return a.printJSON(out)
	}
	a.printf("%s (%s%s)\n", plural(len(out.Portraits), "portrait"), paidNote(out.MicrocreditsCharged, out.MicrocreditsFree), replayNote(out.Replayed))
	first := ""
	for _, p := range out.Portraits {
		if p.Unavailable {
			a.printf("  %s  %s  used or expired\n", p.Upload, p.Provider)
			continue
		}
		if first == "" {
			first = p.Upload
		}
		a.printf("  %s  %s  until %s\n", p.Upload, p.Provider, stamp(p.ExpiresAt))
		if p.Url != nil {
			a.printf("    %s\n", *p.Url)
		}
	}
	a.noteFailed(out.FailedProviders)
	if first != "" {
		a.printf("Use one: atmee avatars create --name NAME --image %s --voice VOICE\n  or: atmee avatars replace-appearance AVATAR_ID %s\n", first, first)
	}
	return nil
}

// paidNote is what a portrait, enhance or voice call cost: the credits
// charged and, when the plan's free generation allowance paid for part or
// all of it, that part (the two add up to the call's price).
func paidNote(charged, free int) string {
	s := credits(int64(charged)) + " credits charged"
	if free > 0 {
		s += " · " + credits(int64(free)) + " from free generation"
	}
	return s
}

// ownReplayNote is the charge note for a clip whose first attempt, under this
// command's own key, ran past the server's wait and was then replayed: the
// clip was charged once, at its size and length's price from the account's
// table, or without a figure when the account doesn't list one.
func ownReplayNote(cl *api.Client, clip api.GeneratedClipBody) string {
	const once = "charged once, when this command's first attempt made it; this answer is that clip, replayed"
	acct, err := cl.Account(ctx())
	if err != nil || acct.Pricing.Generation == nil || acct.Pricing.Generation.ClipPrices == nil {
		return " (" + once + ")"
	}
	if m, ok := clipPrice(*acct.Pricing.Generation.ClipPrices, clip.Width, clip.Height, clip.DurationSeconds); ok {
		return fmt.Sprintf(" (%s credits %s)", credits(int64(m)), once)
	}
	return " (" + once + ")"
}

// clipPrice is a clip's price in microcredits from the account's table: the
// first length tier the clip fits, at its size.
func clipPrice(table []avatarapi.ClipSizePrices, width, height, seconds int) (int, bool) {
	for _, size := range table {
		if size.Width != width || size.Height != height {
			continue
		}
		for _, p := range size.Prices {
			if seconds <= p.UpToSeconds {
				return p.Microcredits, true
			}
		}
	}
	return 0, false
}

func replayNote(replayed bool) string {
	if replayed {
		return "; the first result, replayed"
	}
	return ""
}

func previewLength(d *float64) string {
	if d == nil {
		return "length not reported"
	}
	return seconds(int(*d + 0.5)).String()
}

func (a *app) noteFailed(providers []string) {
	if len(providers) > 0 {
		a.note("No result from: %s\n", strings.Join(providers, ", "))
	}
}
