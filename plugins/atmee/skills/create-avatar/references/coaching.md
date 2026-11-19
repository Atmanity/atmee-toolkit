# Coaching a weak result

## Contents

- When to coach
- Read the symptom
- Offer the fixes
- Another seed
- Fixes that rebuild
- Photo tips for Atmee V2

This is for Atmee V2 and V2 HD avatars. An Atmee V1 avatar has no seeds and
no motion setting, so none of this applies to it. A video clip that looks off
is coached in the `make-video` skill.

A creator who sees one bad seed often decides the model doesn't work. Most of
the time the fix is another seed, a small change to the photo, or a livelier
voice. Help them get there.

## When to coach

1. **As the first build starts,** or at the latest when you first show its
   take: one line, "Each preview is one random take of Mira, so if hers looks
   off, I'll suggest fixes." It is a statement, not a question, and it never
   holds up the build. Say it once per avatar.
2. **After you check a seed** and its lips move weakly or not at all
   (the candidates reference, Check the seed).
3. **Whenever the creator says a preview or a conversation looks or sounds
   off,** while creating, after hand-over, or in a resumed session.

## Read the symptom

You judge one thing yourself: whether the lips move, from the contact sheet.
Everything else in this table reaches you from the creator. Without a
contact sheet, or for anything about sound or timing, say you can't tell from
here and ask what they see or hear. Never invent a verdict.

| Symptom | Shows in | Likely cause | Fixes, cheapest first |
|---|---|---|---|
| Lips barely move | the seed, a conversation | the take; then the photo (lips closed or hidden, a small or turned face) or a flat, quiet voice sample | another seed; a livelier preview line; a photo tweak; a more expressive voice |
| Lips move but out of time | the seed, a conversation | the take; then the photo (lips closed or hidden, a small or turned face) or the voice | another seed; a photo tweak; a more expressive voice |
| Face warped, or drifts from the person | the seed, a conversation | the take; then the photo (face turned away, hands near the face, lips hidden); then the voice | another seed; a photo tweak; a different voice |
| Artifacts: blur, stray objects, things appearing in empty background | the seed, a conversation | the take; then the photo (plain or busy background, small face) | another seed; a photo tweak |
| Only the head moves, no gestures | the seed, a conversation | a head-only photo | an upper-body photo |
| Stiff or jittery movement while talking | a conversation | the motion setting, or the take; then the voice | a different motion setting; another seed; a different voice |

The motion setting is free and applies from the next conversation (the
test-and-handover reference, Movement while it talks).

## Offer the fixes

1. Say the symptom and its likely cause in one plain line, about the avatar,
   not your checks (rule 10): "Mira's lips barely move in this take."
2. Offer two or three fixes from its row, cheapest first, each with what it
   costs, then "Keep it as it is". Another seed comes first whenever its row
   lists it first, unless rule 8's stop has taken it off (Another seed,
   below).
   Ask with choices (rule 9), your pick first: the cheapest fix not yet
   tried for this symptom. A paid fix is never your pick while a free one
   is left.
3. Give the photo and the voice two ways each, free first:
   - "Retake or crop the photo yourself (free)", or a paid change that fits
     the cause: an enhance (no prompt) for poor light, blur, low resolution
     or a small face; a restyle of the same photo with a prompt naming the
     change for pose, lips, framing or background ("facing the camera, a
     slight smile, chest-up, a soft plain backdrop"), since a plain enhance
     keeps all of those (the credits-and-generation reference, Enhancing or
     restyling the creator's photo);
   - "Record a livelier voice sample (free)" or "I design a new voice
     (<price>)".
4. A paid pick is not the yes. Ask the price-and-balance question for that
   call first (rule 1, the credits-and-generation reference).
5. If the creator keeps it as it is, carry on. Raise the same symptom again
   only if they ask.
6. **When a fix didn't help,** the cause is likely on another side. Don't
   offer more of the same kind: after a new take or a new line, name the
   photo and the voice as the likely causes and offer both. Look at the
   photo again against the tips below (and the portrait checks) and say what
   in it could be the cause; "her lips are closed in the photo" is more use
   than "try another voice". The menu keeps the same shape as above: free
   fixes first, your pick free, then "Keep it as it is".
7. **When the photo tweaks didn't help either,** suggest a different photo
   altogether: another photo of the same person, or for an invented
   character a new generated portrait (paid, asked first).

For a creator who wants to read more, the guide is at
`https://www.atmee.ai/docs/guides/optimize-your-avatar`.

## Another seed

Each seed is a random take of the same photo, voice and line, and one take
can be much better than another. Only when the creator picks it, make one
more with `add_candidates` (`count` 1, within the free slots under Limits in
the candidates reference), judge it the same way, and show both. Never add
seeds on your own, not even to correct something: if what the server shows
differs from what you told the creator, say so plainly and offer the fixes
again. Adding a seed never changes the
serving one: the creator chooses (rule 11).

Rule 8's stop counts every capture run as a batch, one added seed included,
and a weak seed counts as failed. After two batches in a row on the same
photo, voice and preview line where every seed was weak or dead, take
"another seed" off the menu and offer a photo or voice change instead. A new
line, photo or voice starts the count again. In a resumed session, count the
batches from `list_candidates` with `versions` `all`.

## Fixes that rebuild

A new preview line (`update_avatar` `previewMessage`, in the avatar's
language, 60 code points or fewer), a new photo or a new voice builds a new
version. The old seed keeps serving until the new version's first seed
finishes, and on an avatar that picks takes automatically (`seedSelection`
`auto` in `list_candidates`, as this skill creates it) that seed then goes
live by itself; with `manual`, the creator picks as usual. Say so in the
choice:
"A livelier line (free; her first new take goes live by itself, and you can
switch back)."

When the new seed is ready, show the old and the new one and ask which to
keep. Switching back selects the old seed (`select_candidate`), which
restores that version's photo, voice and line in one step; nothing is
captured or charged, and the character stays as it is now. Say that before
you switch.

## Photo tips for Atmee V2

- Chest-up or half body works best.
- A head-only photo gives no hand or body gestures: suggest an upper-body
  photo of the same person, or a restyle of theirs asking for chest-up
  framing. A generated half-body portrait (paid, asked first) is for an
  invented character only, never a real person (rule 4).
- When the face is small, zoom in or crop closer, but not closer than
  chest-up.
- Keep the face turned to the camera: turning away brings distortion and
  drift from the person's look.
- Keep the lips visible, not closed, covered or behind a filter. A slight
  smile with some teeth showing works well; avoid a wide grin or an open
  mouth, which repeat in every frame.
- Avoid a plain white or empty background, and a busy one with other people.
- Even, soft light on the face and a relaxed, upright posture help.
