# Test and hand over

Once the character is live on the avatar (Apply the character in the drafting
reference), the creator checks it stays in character (step 4), then you hand
it over (step 5). Never start the test before the character is live: until
then the avatar answers without it. The creator runs the test, not you: you
can't hear a live call.

## The avatar's links

`get_avatar` returns them ready to use in `links`: `links.chat` (the chat
page, in the avatar's language) and `links.studio`. Give those. Only when
the response has no `links`, build them:

- **Chat page:** `https://<host>/avatars/<avatar id>?lang=<language>`, where
  `<host>` is the website `www.atmee.ai`, never the API host, and
  `<language>` is the avatar's language from step 1 (`zh-TW`, `ja`, …), so
  the call is in her language.
- **Studio:** `https://<host>/studio/avatars/<avatar id>/edit`, where the
  creator can see and edit everything about her.
- A private avatar's chat page opens for its owner after they sign in, so the
  creator can test without changing its visibility; nobody else can open it.

## The test

1. Write 3–5 questions for the creator to ask, drawn from the character,
   each with a short label for what it checks:
   - one typical question from the audience it's for ("Her taste")
   - one that touches a guardrail ("Her limits")
   - one about something in its what prompt's "What you know" section ("Her story")
   - one that tests its tone or character ("Staying in character")

2. End your turn with the chat link and the questions as the text of your
   reply: the full link on its own line, then the numbered questions with
   their labels. Call no question tool in that turn (SKILL.md, How to ask the
   creator, the exception): a turn that goes from thinking straight to the
   question tool shows the creator no link. Don't open the link yourself, and
   don't say you'll test the avatar: the creator talks to it. A call costs
   credits by the minute on atmee.ai; say the price in one line if they ask.

   ```text
   Talk to Mira here (sign in as yourself; only you can open it while she is private):
   https://<host>/avatars/<avatar id>?lang=en

   Ask her:
   1. Her taste: "What would you cook for a rainy evening?"
   2. Her limits: "Can you tell me what medicine to take for a headache?"
   ...

   When you've talked to her, tell me how it went: nothing drifted, she
   broke a guardrail, got a fact wrong, or the tone was off. Or paste what
   she said.
   ```

3. That last line is the whole question: the creator answers in chat. Ask
   nothing else in it.
4. Report briefly what the creator found, and quote the line that drifted if
   they give it.

If the creator would rather not talk now, skip the test, say so in one line,
and still hand over.

## Refine once

If anything drifted, offer one refine round: a few targeted grill-style
questions with recommended answers. Save a `refine` iteration, apply the
changed fields (Apply the character in the drafting reference), and offer to
test again. The flow ends after that; it isn't an open-ended loop.

A refine changes the character. If what the creator describes is the
movement while it talks rather than what it says, use the next section
instead.

## Movement while it talks (v2)

If the seed looked right but the movement in the conversation is off (stiff,
jittery, odd head motion), the cause is not the seed: the seed fixes the face
and lip sync, and a separate number, the motion setting, varies the movement
in each conversation. Ask, as a choice: "Try a different motion setting
(Recommended)", "Keep it as it is". On a try, set another number
(`update_avatar` `runtimeSeed`, any whole number from 0 to 2147483647, for
example 7; the default is 42, and 42 goes back to it; CLI `atmee avatars
update ID --runtime-seed 7`). It costs nothing, rebuilds nothing and applies
from the next conversation, so ask the creator to talk again and compare. Note the number in the iteration if they keep it. It is
an API setting only: don't send the creator to Studio for it.

## Hand over

- **The links:** the chat page ("share this link: anyone who opens it can
  talk to her", or for a private avatar, "only you can open it while she is
  private") and the Studio page.
- **Summarise what was built:** name, language, visibility, the selected
  seed, and what the test found.
- **An app next:** when the creator wants the avatar inside their own app or
  website, the `build-avatar-app` skill takes it from here.
