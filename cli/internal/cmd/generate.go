package cmd

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/Atmanity/toolkit/cli/internal/api"
	"github.com/Atmanity/toolkit/cli/internal/avatarapi"
)

// Generation answers with upload:<uploadId> references, which create and the
// replace commands take as they are.

const generationNote = "Costs the operation's price (`atmee account`), charged only when something comes back; a flagged, failed or refused " +
	"call costs nothing. Results last 24 hours and work once. A signed-in user, or an API key with generation:write and avatars:write."

const idempotencyHelp = "Reuse the key a failed call printed to retry it: the same key never charges twice and returns the first result"

func (a *app) generateCmd() *cobra.Command {
	return group("generate", "Make portraits and voices for an avatar, or a video clip (portraits, enhance, voice, clip)",
		a.generatePortraitsCmd(), a.generateEnhanceCmd(), a.generateVoiceCmd(), a.generateClipCmd())
}

func (a *app) generatePortraitsCmd() *cobra.Command {
	var description, key string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "portraits --description TEXT",
		Short: "Generate portraits of an invented character from a description.",
		Long: "Generate up to four square portraits of an invented character, one from each image provider. Never use it to depict a " +
			"real person from a description: for a real person, with their permission, use `atmee generate enhance` on their own photo. " + generationNote,
		Example: `  atmee generate portraits --description "a weathered lighthouse keeper, sixties, warm smile"`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(description) == "" || utf8.RuneCountInString(description) > 2000 {
				return usagef("--description takes 1 to 2000 characters: what the portrait shows")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			out, err := cl.GeneratePortraits(ctx(), description, key)
			if err != nil {
				return err
			}
			return a.printPortraits(out, jsonOut)
		},
	}
	f := c.Flags()
	f.StringVar(&description, "description", "", "What the portrait shows, an invented character (required)")
	f.StringVar(&key, "idempotency-key", "", idempotencyHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) generateEnhanceCmd() *cobra.Command {
	var image, prompt, key string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "enhance --image PATH",
		Short: "Enhance a photo, or restyle it with --prompt.",
		Long: "Crop a photo to a square and return up to four portraits, one per image provider. Without --prompt they are enhanced " +
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
			out, err := cl.EnhancePortrait(ctx(), ref, prompt, key)
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
			a.printf("%s in %s (%s credits charged%s)\n", plural(len(out.Voices), "voice preview"), out.Language,
				credits(int64(out.MicrocreditsCharged)), replayNote(out.Replayed))
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
	var resolution, key string
	var seed int64
	var jsonOut bool
	c := &cobra.Command{
		Use:   "clip --image PATH --prompt TEXT",
		Short: "Render a video clip from an image and a prompt.",
		Long: "Render an mp4 of 1 to 15 seconds of the person or character in the image, moving as the prompt says, silent or speaking " +
			"--audio. No avatar is made or needed. --duration is required with --audio (it is not read from the file) and defaults to 8 " +
			"seconds for a silent clip. If the image shows a real person other than you, have their permission first. A clip is " +
			"priced per 15 seconds of video, prorated (`atmee account`), the same with or without audio, charged only when it comes back; a flagged, failed or refused call costs nothing. " +
			"The result lasts 24 hours. Needs v2 access. A signed-in user, or an API key with generation:write and avatars:write.",
		Example: `  atmee generate clip --image ./me.jpg --prompt "she walks along the beach at sunset"
  atmee generate clip --image upload:ID --prompt "a slow nod" --audio ./line.wav --duration 6 --resolution 384x672`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := cmd.Flags()
			if in.Image == "" {
				return usagef("--image is required: a local jpg, png or webp, an https URL or upload:ID")
			}
			if strings.TrimSpace(in.Prompt) == "" || utf8.RuneCountInString(in.Prompt) > 2000 {
				return usagef("--prompt takes 1 to 2000 characters: the shot and the motion")
			}
			for flag, v := range map[string]string{"--image": in.Image, "--audio": in.Audio} {
				if strings.HasPrefix(v, "http://") {
					return usagef("%s: asset URLs must be https:// (%s)", flag, v)
				}
			}
			if in.Audio != "" && !f.Changed("duration") {
				return usagef("--duration is required with --audio: the audio's length in seconds, at most 15")
			}
			if f.Changed("duration") && (in.Duration < 1 || in.Duration > 15) {
				return usagef("--duration takes 1 to 15 seconds")
			}
			if f.Changed("resolution") {
				r, err := api.ParseResolution(resolution)
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
			local := !api.IsRemote(in.Image) || in.Audio != "" && !api.IsRemote(in.Audio)
			if key != "" && local {
				return usagef("--idempotency-key retries the same inputs: give --image and --audio the upload:ID the failed call printed, not the file again")
			}
			cl, err := a.api()
			if err != nil {
				return err
			}
			var sent []string
			if !api.IsRemote(in.Image) {
				if in.Image, err = cl.UploadPortrait(ctx(), in.Image); err != nil {
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
				if errors.As(err, &apiErr) && apiErr.Code == "v2_access_required" {
					return fmt.Errorf("%w; clips need v2 access: %s", err, redeemV2Hint)
				}
				return err
			}
			if jsonOut {
				return a.printJSON(out)
			}
			clip := out.Clip
			a.printf("Clip of %s at %dx%d (%s credits charged%s)\n", seconds(clip.DurationSeconds), clip.Width, clip.Height,
				credits(int64(out.MicrocreditsCharged)), replayNote(out.Replayed))
			if clip.Unavailable {
				a.printf("  %s  expired\n", clip.Upload)
				return nil
			}
			a.printf("  %s  until %s\n", clip.Upload, stamp(clip.ExpiresAt))
			if clip.Url != nil {
				a.printf("    %s\n", *clip.Url)
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&in.Image, "image", "", "Who or what the clip shows: a local jpg, png or webp (up to 10 MB), an https URL or upload:ID (required)")
	f.StringVar(&in.Prompt, "prompt", "", "The shot and the motion, up to 2000 characters (required)")
	f.StringVar(&in.Audio, "audio", "", "What the clip says or plays: a local audio file, an https URL or upload:ID; without it the clip is silent")
	f.IntVar(&in.Duration, "duration", 0, "Length in seconds, 1 to 15: required with --audio, set to the audio's length; default 8 for a silent clip")
	f.StringVar(&resolution, "resolution", "", "The frame size as `WxH`, e.g. 512x512 (default, square), 384x672 (portrait) or 672x384 (landscape): see `atmee account`")
	f.Int64Var(&seed, "seed", 0, "The render's seed; another seed gives other motion")
	f.StringVar(&key, "idempotency-key", "", idempotencyHelp)
	f.BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) printPortraits(out *api.GeneratedPortraits, jsonOut bool) error {
	if jsonOut {
		return a.printJSON(out)
	}
	a.printf("%s (%s credits charged%s)\n", plural(len(out.Portraits), "portrait"), credits(int64(out.MicrocreditsCharged)), replayNote(out.Replayed))
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
