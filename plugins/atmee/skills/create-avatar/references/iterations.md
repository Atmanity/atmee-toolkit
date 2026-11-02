# Iteration files

## Contents

- Where
- Assets in the files
- What each file records
- Commits
- Resume
- Without a filesystem

Every step is saved, so any version of the avatar can be rebuilt and nothing
tried is lost.

## Where

One folder per avatar under `atmee-avatars/` in the current project:

```text
atmee-avatars/
  settings.json             # the accountId setup saved, when there is one
  mira-k3f9/                # <slug>-<4-character id>, fixed when the avatar is named
    000-create.md
    001-candidate-selected.md
    002-draft.md
    003-grill-layer-1.md
    004-test.md
    sources/                # the creator's own files, copied in as they arrive
    media/                  # saved portraits, voice samples and seeds
```

- Make the folder as soon as the avatar has a name, before the create.
- Make the slug from the name in plain lowercase ASCII letters, digits and
  dashes (for 小美 use a romanisation like `xiaomei`, or `avatar` if there is
  none), then add 4 random lowercase characters. Don't rename the folder later,
  even if the avatar's name changes. The name lives inside the files.
- Number files in order: `NNN-<step>.md`. Steps are `create`,
  `candidate-selected`, `draft`, `grill-layer-1|2|3`, `refine`, `update`,
  `replace-portrait`, `replace-voice`, `test`. The create comes first: step 1
  makes the avatar before its character exists.
- A `draft`, `grill-layer-N`, `refine` or `update` file records `applied`:
  `no` when saved, `yes` once the change is live on the avatar (Apply the
  character in the drafting reference). Resume reads it.
- Use `templates/iteration.md` for each file.

## Assets in the files

Don't ask the creator how assets are stored; record them this way:

- **Generated portraits and voices:** the upload id from its
  `upload:<uploadId>` (`upload_id`) and its `provider`. It works for one
  avatar and expires 24 hours after it was made, and its link sooner, so the
  saved copy under `media/` is what you show later. Never store its link.
- **The creator's own files** have no link to refresh: the copy in
  `sources/` is the reference. When one is uploaded for a create or replace,
  store its upload id with that version's inputs (`upload_id`). It worked once
  and can't be fetched again; a later version uploads the `sources/` copy
  afresh. Never store an upload link.
- **Saved copies:** anything you open for the creator to review is saved
  under `media/` with `scripts/media.sh save`, because a server link expires
  before the review is over. Record that path as the asset's `copy`.

## What each file records

- **Front matter:**
  - the environment: mock or real, plus the server URL
  - the avatar id (once created)
  - the assets used
  - the real-person permission answer, if any
- **Body:** the full spec as it stands, what changed in this step, the inputs
  used, and a one-line note.

Each file must be enough on its own to rebuild that version.

## Commits

Never offer a commit or `git init`; the files are saved without one. When
the creator asks to commit the avatar's folder:

1. If the avatar is a real person, warn first that the files describe them.
2. If the folder isn't in a git repository, ask before running `git init`.
3. Keep `media/` out of git before you `git add` anything: add
   `atmee-avatars/**/media/` to `.gitignore` if it isn't there. It holds the
   creator's photos and voices. If it was tracked before, untrack it with
   `git rm -r --cached`.
4. Stage only `atmee-avatars/<folder>/`, `atmee-avatars/settings.json` and the
   `.gitignore` change, never other changes in the project. Check that nothing
   under `media/` is staged.

## Resume

When the skill starts and `atmee-avatars/` has avatars:

1. List them: name, where it stands (below), avatar id, environment. A
   folder with no iteration file yet was named but not created: read the name
   back from the folder's slug and confirm it with the creator. `clips` holds
   standalone clips and is never an avatar, so skip it.
2. Offer to continue one from where it stands, or start a new one.
3. When continuing, read the latest file. If its `paid_calls` has a
   `status: pending` entry, retry it first with the same `request_id`,
   identical inputs and `confirm: true`, without asking again (the credits
   reference, A pending entry left behind). Then fetch the avatar from the
   avatars server. The server is the source of truth for live fields; the
   file is the source of truth for what the creator intended. Show any
   difference (for example, it was edited in Studio, or it's still building)
   and ask which to keep.

Where an avatar stands, from its newest files and `get_avatar`. A folder
begun before 0.9 has drafts with no `applied` field; there the `create` file
decides: if its spec carries a who prompt (`personality` in older files), the character went in with the
create and those drafts count as applied; if it carries none, the newest
draft is not applied yet:

| It stands at | Because | Offer |
|---|---|---|
| named, not created | no `create` file with an avatar id | step 1's create |
| created, still building | an avatar id, no serving seed | wait for it, then the seed |
| live, no character yet | a serving seed, no `draft` file | the character (step 3) |
| character drafted, not applied | the newest draft, grill, refine or update file says `applied: no` | apply it once the seed serves (the drafting reference); with no serving seed, the seed (step 2) first |
| character applied, not tested | the newest such file says `applied: yes`, no `test` file | the test |
| tested | a `test` file | hand-over, or one refine round |
4. If the file says mock but the plugin now points at the real server, say the
   avatar only exists on the mock and offer to create it again on the real
   server from the latest iteration.

## Without a filesystem

In claude.ai chat, post a short iteration summary after each step instead of
writing a file: the step, what changed, and the spec's key fields. Before a
paid call, add its pending `requestId` and exact inputs to the summary, and
retry a pending one from an earlier summary the same way before any new paid
call.

To resume there, list the creator's avatars (`list_avatars`) and read the one
they pick with `get_avatar`: no serving seed means it is still building or
needs its seed fixed (step 2); a serving seed with no `persona.whoPrompt` means
it has no character yet (step 3); a `whoPrompt` with `stages.persona` not
yet `completed` means the character is still going on. Ask the creator for
anything the server doesn't keep, such as the character draft they had not
applied.
