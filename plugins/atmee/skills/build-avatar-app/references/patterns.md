# App shapes

## Contents

- 1. Guided conversation
- 2. Lesson and quiz
- 3. Companion that watches
- 4. Narrator driven by events
- 5. Broadcast host
- Tools in any shape

Five shapes, each built as a sample app on the Avatar API and run in
live sessions. Pick the closest, say which in one line, and build to its recipe;
most apps are one of these or two combined. Each also meets the limits in
SKILL.md's "Current limits"; **Expect** names the one that bites it.

When you can reach them (they are not public yet), the Atmee demo apps,
`https://github.com/Atmanity/atmee-ai-demos`, hold
these five and fourteen more (games, tutors, advisors), each with an
`apps/<app>/README.md` on how it uses the API and what it hit. Read them for
working patterns; build what this app needs.

## 1. Guided conversation

A recruiter's screening call, an onboarding, a sales discovery call.

- **User:** microphone, with typing as a fallback.
- **Agenda:** a conductor, one step per topic (`references/conductor.md`).
- **`state`:** the facts the conversation is about (the job description, the
  product) and who the user is ("Candidate name: ..."), sent once linked and
  again whenever they change.
- **Page:** the agenda's progress, from the avatar's captions; a transcript.

## 2. Lesson and quiz

Internal training, a course, a product walkthrough.

- **User:** listens, then asks and answers by voice or typing.
- **Agenda:** `speak` edges through the lesson steps, a questions step that
  waits, quiz steps whose verdicts start with "Correct" or "Incorrect".
- **`state`:** the slide on screen and the score so far; the app keeps score
  from the verdicts in the captions and sends the new state.
- **Page:** slides that follow the avatar and that the user can also turn
  (each turn is a new `state`). The avatar turns them with a `show_slide`
  tool (notify), or, without tools, the page matches "slide two" in its
  captions. A `get_score` tool (await) lets it read the score instead of
  waiting for the next state.

## 3. Companion that watches

Sits next to the user and reacts to what they do: a coding buddy, a study
partner, a shopping assistant.

- **User:** mostly busy; talks or types now and then.
- **The app's eyes:** its own vision model, run by the backend, on screen or
  camera frames when they change. The demo asked a Gemini model for one
  sentence of what the user is doing and, separately, anything noteworthy.
- **`state`:** the latest description, every time.
- **`event`:** only the noteworthy changes (an error, a finished task),
  `ttl_s` about 15, with "react briefly if it deserves a word".
- **Expect:** the check-in on quiet stretches; a `state` that says "speak
  only when something noteworthy happens" keeps it brief, and `checkInAfterS`
  (`server.md`) sets it or turns it off.

## 4. Narrator driven by events

A game, a sports or market commentary, a guided story.

- **User:** `mute` from the start; the avatar speaks only when the app tells
  it something happened.
- **`event`:** every move, `ttl_s` about 12, written as the fact plus the
  tone ("in one or two vivid sentences").
- **Talking allowed in places:** `unmute`, a `state` that says the user may
  talk now and how to answer, an `event` that invites them; `mute` and a new
  `state` when they leave.
- **Expect:** the check-in ("Still there?") between events; keep events
  frequent, or start the session with `checkInAfterS: 0` (`server.md`).

## 5. Broadcast host

A live stream or a webinar where the avatar reads an audience's chat.

- **The page in the room** is the host's: the only participant. The audience
  watches a restream (OBS, or re-publishing the avatar's tracks into the
  developer's own LiveKit room).
- **User:** `mute`; the avatar hears nobody, it reads.
- **Audience chat** goes to the backend: a direct question becomes a typed
  turn framed with the name ("Mia asks: ..."); everything else is batched into
  one `event` about every 20 seconds (`ttl_s` 30), with "react to the chat in
  a sentence or two".
- **`state`:** the viewer count and the last few chat messages.
- **Captions:** the backend relays the avatar's words to the audience's chat.
- **Expect:** the session length limit; a longer stream chains sessions.

## Tools in any shape

Tools are for what the avatar makes the app do or fetch; what the app tells
the avatar stays `state`, `note` and `event`.

- **Fit them to the avatar's role.** The persona outweighs tool instructions:
  in testing, an avatar made as a chess player refused to "show slide 3" even
  when told to use the tool. Name and describe tools as things this avatar
  would do ("put the next lesson slide up", not "trigger UI update 3"); when
  the role itself is wrong for the app, change the persona or the avatar.
- **Few, clearly named tools,** each one action with a description that says
  when to call it; keep them distinct, so there is one obvious tool for each
  request.
- **One `instructions` text for all of them:** when to call which, in the
  avatar's terms ("Put each slide up before you teach it; when the trainee
  asks how they are doing, get the score"). There is no per-tool
  instruction beyond its description.
- **`notify` for actions, `await` for answers.** An `await` call holds the
  conversation: the avatar says at most one short filler line while it runs,
  so keep the page's handler fast and its `timeoutSeconds` small.
- **The app decides, the avatar performs.** Rules, scores and facts live in
  the app; a result is the outcome first ("Done: ...", "Not yet: ..."), then
  how to say it, so the filler spoken while an `await` runs can't frame the
  verdict.
- **Lenient schemas:** no `enum`, `pattern` or `additionalProperties:
  false`. A call whose arguments miss the schema never reaches the page,
  and the avatar often says the action happened anyway; list the accepted
  words in the description and normalise them in the app.
- **Narrated, not called:** now and then the avatar says it did something
  without the call. For actions safe to repeat (next, start over), the page
  can match the user's request and act itself after a few seconds, then
  tell the avatar with an `event`.
