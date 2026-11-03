# Portrait checks

Check every portrait before you use it: the creator's own photos, and the
portraits you generate or enhance. The aim is to catch photos that will
animate badly, never to steer the creator's style or framing. A v2 avatar
takes close-ups, half-body and full-body shots, photos, paintings and cartoons
alike, as long as the face is big enough in the centre crop and has a
visible, uncovered mouth and jaw. A wider shot shows more of the body moving
as the avatar talks, so never ask for a closer photo because of the framing
alone. The `create_avatar` tool's own text still says "head and shoulders,
face at least about a third of the frame": that is out of date for v2, so
follow this file and don't repeat it to the creator of a v2 avatar.

A v1 avatar is stricter: front-facing, the face at least about a third of the
frame, lips slightly parted. Say so when you make v1. Both engines take jpg,
png or webp of at most 10 MB.

Warn, don't block: the creator may use any photo they choose.

## 1. Measure

Where you can run commands, run `scripts/portrait-info.sh <image>`. In
claude.ai, run the Python line it prints instead. Compare with the `measure`
block in `negative-examples/manifest.json`:

- too small: on the default square frame, short side under `min_side`; on a
  chosen non-square frame, width or height under the frame's. Say so (no need
  to look further for this)
- a format not in `formats`, or bigger than `max_bytes`: say the server won't
  take it
- `aspect` above `aspect_borderline`: the engine keeps only the centre crop of
  the avatar's frame (a square unless the creator chose a portrait or
  landscape aspect ratio), so look at where the face sits (step 2, framing). A
  tall photo for a portrait frame, or a wide one for landscape, is fine

`bytes_per_pixel` is for your information only: a clean photo with a plain
background also compresses small. Never report compression from the number.

## 2. Look

Look at the photo once, against this list. Most photos pass.

| Group | Problems |
|---|---|
| Framing | head cut by the edge; face near the edge of a wide photo; face too small in the centre crop (under about a fifth of its width); a face filling the whole frame with no hair outline or neck; tilted |
| Mouth and face | anything over the mouth or jaw (hand, cup, phone, mask, scarf, microphone); hair over the face; beard or moustache hiding the lips; sunglasses or strong glare; a hat shading the eyes; no visible mouth at all (full mask, helmet, mascot head) |
| Pose and expression | profile or near-profile; eyes looking away; a strong expression (mouth wide open, shouting, tongue out); a close wide-angle selfie that distorts the face |
| Quality | too small or upscaled; blurry; heavy compression; a photo of a screen or a print |
| Light | washed out; backlit; strong coloured light; a hard shadow across the face |
| Other content | a collage or grid; filters or AR stickers; another face in the photo; text, a watermark or a logo over the face; a background the same colour as the hair |

Each problem's severity, what to look for, why it fails and what to ask for
are in `negative-examples/manifest.json`. Problems without an image there
(profile, masks and helmets, mascot heads, wide-angle selfies, text over the
face, a background matching the hair) are judged from this list alone.

## 3. Open an example only when unsure

When a photo might have a problem but you can't tell, open that problem's
example image (`file`, and `crop_result` for the framing pairs) and compare.
Every example is made from the same head-and-shoulders photo, only to show
its one problem: that framing is not the target, and a half-body or
full-body shot without the problem passes.
Don't open examples for photos that are clearly fine or clearly not.

## 4. Tell the creator

- Most severe first (`blocks`, then `likely-fails`, then `may-work`), one line
  each: what you saw and the `ask_creator` fix, in your own words.
- For `may-work`, say it may still work and offer to try it.
- Never mention the example images, and never comment on style, looks or
  attractiveness.
- If nothing is wrong, say nothing about these checks.
