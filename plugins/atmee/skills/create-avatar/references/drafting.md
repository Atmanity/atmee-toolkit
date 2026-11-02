# Openers, the character draft and applying it

## Contents

- Step 1: the two openers
- Check the portrait and voice
- Step 3: read everything first
- What the draft covers
- Show the draft
- Apply the character

## Step 1: the two openers

Step 1 asks nothing about the character: the creator sees the avatar talk
first. Ask these in one call of your question tool (SKILL.md rule 9), in the
creator's language:

| Question | Options |
|---|---|
| **Who is the avatar?** | Me · Someone I know · A brand representative · A made-up character |
| **What do you already have?** (several allowed) | A portrait · A voice sample · Documents or links · Nothing yet |

If the answer to the first is a real person other than the creator, ask for
their permission now (SKILL.md rule 4), before any name, file, generation or
create. Keep the answer and write it into the `create` iteration file.

Then ask for what step 1 still needs, in one more call: the avatar's name,
its language (the creator's own language first, marked "(Recommended)"), and
the files they said they have ("I'll drag them into the chat now
(Recommended)" or "Go ahead without them"). Offer your best guess for the
name. Skip whatever their answers already gave you. Say once that the
language is what the seed speaks, so changing it later makes new seeds.

The language is one of `en`, `zh-TW`, `zh-CN`, `ja`, `ko`, `fr`, `vi`, `es`,
`de`, `hi`, `ru`.

## Check the portrait and voice

Check what the creator has against these rules. Warn, don't block.

- **Portrait:** run the portrait checks (the portrait-checks reference). In
  short: at least the avatar's frame on each side (the engine keeps the centre
  crop of that frame, 512×512 unless another resolution is chosen), one person
  facing the camera, the mouth and jaw visible and uncovered, lips slightly
  parted or a relaxed smile; jpg, png or webp up to 10 MB. A v2 avatar takes a
  wide range of shots and styles: a close-up with the shoulders, a half-body
  or a full-body photo, a photo, a painting or a cartoon. Only the face has to
  stay big enough in the centre crop: narrower than about a fifth of it,
  the server refuses the portrait (`portrait_face_too_small`). Use whatever
  framing the creator gives you.
- **Voice sample:**
  - 10–60 seconds of one person speaking clearly, with no music or other voices
  - an expressive, energetic delivery animates noticeably better than flat reading
  - up to 12 MB
- **The pair:** the same voice can work on one face and not on another.

If the creator has no portrait or voice, offer to generate one now (the
credits-and-generation reference): step 1 can't create the avatar without
both.

## Step 3: read everything first

Documents and links the creator gave in step 1 feed the character. Read every
one before asking anything. Never ask for a fact that is already in them: if
a bio says where the avatar grew up, don't ask where it's from. Ask the two
character openers only where the material doesn't answer them:

| Question | Options |
|---|---|
| **Who is <name>, in a line? Pick one or type your own** | your best guess (Recommended) · a second guess |
| **What tone should <name> have?** | Warm and casual · Crisp and expert · Playful |

## What the draft covers

Write each field to fit the Avatar API's limits. These are stricter than
Studio's in places, and the API is what accepts or rejects the change.
`update_avatar` states its limits itself; if it and the table ever disagree,
the tool wins.

| Field | Limit | Notes |
|---|---|---|
| preferred alias | up to 100 | what the avatar is called in conversation |
| spoken name | up to 100 | how the name is pronounced, if it differs |
| known aliases | up to 20, each up to 100 | nicknames people may use |
| who prompt (`whoPrompt`) | 1–10,000 | who the avatar is (Studio's "Who are you") |
| what prompt (`whatPrompt`) | 1–10,000 | how it treats the person it talks to, its mission, and what it knows (below; Studio's "What are we doing") |
| greeting | up to 500 | its first message |
| self-introduction | 51–2,000 | how it introduces itself |
| guardrails | up to 20, each up to 500 | what it must never say or do |

If source material runs over a limit, shorten it and tell the creator you did.
A 3,000-character self-introduction becomes one under 2,000.

### What it knows

A v2 avatar's knowledge goes into its what prompt, under a heading of its own,
so `update_avatar` can change it any time:

```text
Treat user as: a customer. Mission: help them choose a tea and answer
questions about the shop.

What you know:
- The shop opened in 2019 on Dihua Street, Taipei.
- Opening hours: 10:00–19:00, closed Mondays.
- ...
```

Put in what the avatar must always know: names, dates, prices, what it
offers, a few paragraphs. From a long document, keep what its audience will
ask about. The whole what prompt fits in 10,000 characters; when the knowledge
won't, keep the most-asked facts, tell the creator what you left out, and
keep the full source in `sources/`.

Never send `knowledge.facts` for a v2 avatar: the API saves it as Creator
Studio's "Facts with context", which v2 conversations don't read yet, so the
what prompt holds the knowledge instead. Knowledge files (`replace_knowledge`)
are v1 only and have no effect on a v2 avatar.

### Templates

Use these shapes, adapted to the avatar:

- **Who prompt:** "Role: You are a friendly companion. Personality: Warm,
  relaxed, and easy to talk to. You genuinely care about how people are doing."
- **What prompt:** "Treat user as: a friend. Mission: Follow the user's lead and
  go wherever the conversation takes you." Or "Treat user as: a student.
  Mission: Find out what the user is trying to learn, and guide them through
  it." Then "What you know:" as above.
- **Greeting:** "Hey there! Really nice to meet you. How are you doing today?"
- **Guardrails:** "Do not provide medical, legal, or financial advice."
  "Always offer to escalate to a human if unsure."
- **Self-introduction:** "Hi, I'm Coach Mike. I help engineers ace their tech
  interviews."

## Show the draft

Show the draft as a readable summary, not raw JSON, and save it as an
iteration with `applied: no` (the iterations reference). Then ask "Put this
character on <name>?" with "Yes, apply it (Recommended)" and "Sharpen it
first" (the grill-layers reference).

## Apply the character

Apply it only once the first seed serves (`servingCandidateId` set,
`readyToChat` true). A persona change is moderated again, and seeds still
being captured wait for that check, so applying earlier would hold up the
seed. If the creator approves the draft before the seed serves, say the
character goes on as soon as the seed is ready, and apply it then.

1. Send the whole character in one `update_avatar` call: whoPrompt,
   whatPrompt (`personality` and `scenario` are their older names; send the
   current ones), selfIntroduction (always, 51–2,000 characters: nothing derives
   it on an update), greeting, guardrails, and the aliases and spoken name
   when you have them. Check every limit in the table before sending.
2. On a `422` or `400`, fix the field it names and send the whole set again.
3. Follow it with `get_avatar` every 10 seconds or so (not `wait_for_avatar`:
   the avatar stays `ready` while its persona rebuilds, and even when the
   persona fails). The character is live only when you have seen this
   update's rebuild start and then finish: a read after the update showing
   `stages.persona.status` `pending` or `processing`, or `moderationState`
   `unmoderated`, followed by `completed` with `moderationState` past
   `unmoderated`. The reads before that are the previous build, often the
   empty one from the create, so they never count. If no start shows within
   two minutes, don't treat the character as live: tell the creator you
   can't confirm it yet, and check again before the test.
4. On `stages.persona.status` `failed`, relay `userFeedback` (from the stage
   or `latestFailure`), rephrase what it points at, and send the whole set
   again; rule 8's two-failures limit applies.
5. On `status` `moderation_hold`, stop: a person reviews it. Tell the
   creator, and don't retry.

Once it is live, mark the iteration `applied: yes`.
Later rounds (the grill, a refine) send only the fields that changed, one
`update_avatar` call per round, and are checked the same way.
