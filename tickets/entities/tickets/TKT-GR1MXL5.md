---
id: TKT-GR1MXL5
type: ticket
title: Structured questions in realtime channels
kind: enhancement
priority: medium
effort: xl
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

## Description

A channel message type can carry options that render as reply buttons
(TKT-03FQPD), but only as a flat string list, one question per message. Richer
asks have to be flattened into fixed slots (`a`, `a_pros`, `a_cons`, `b`, ...),
and several related decisions need several round trips, each of which can move
the session on its own.

Make structured questions a generic capability of message types. Nothing here is
specific to agents or to planning decisions. An agent asking a person to pick
between designs is one use; a group vote or a check-in with several picks are
others.

## Design decision: one message per question, grouped by batch

Several questions are several ordinary question messages, posted together as one
batch, not one message holding nested containers. Each answer is its own
message, posted on click. Grouping is a UX nicety on top of flat messages, so it
works the same on every device and through the API, MCP and the CLI, and it
needs no custom container shape per use case. Partial answers are stored
messages, so they survive a reload and show on every device. (Nested containers
and flat lists linked by a group id were considered and rejected with the user
on 2026-11-11.)

The session carries the "all answered" logic. An answer to one of several open
questions moves nothing; the last answer satisfies a transition guard. An agent
therefore waits exactly as today: it watches the session and wakes once, when
the session moves. No harness-side blocking is needed.

## Scope

1. **Object list properties on message types.** One level only:

   ```yaml
   options:
     type: object
     list: true
     properties:
       label: { type: string, required: true }
       pros: { type: string, list: true }
       cons: { type: string, list: true }
       recommended: { type: boolean }
   ```

Sub-properties take the scalar types and string lists, never another object.
Values are validated per item on post.

2. **Choices from an object list.** `choices: { from: options, label: label, ... }`
reads the button text from the named sub-property and renders one button per
item, on that item's card when the type shows cards.

3. **Batch post.** A poster may post up to 10 messages in one step, all or
nothing, with a shared batch id. For the session a batch is one post: every
message is admitted against the state before the batch, and the batch moves the
session at most once. HTTP, MCP and the CLI each get a batch form.

4. **Asked questions.** Every message with `choices:` in a batch is asked in
the span that holds after the batch. An answer of the declared answer type that
replies to it counts; removing that answer uncounts it. A transition guard reads
the number still unanswered, e.g. `answer: { to: working, when:
"session.open_questions == 0" }`. Buttons show on every unanswered asked
question. A move clears the set.

5. **Grouped display.** The web app shows a batch of questions as one group:
"Question k of N", answered questions collapsed to the chosen value with
"Change" (edit or remove the own answer, as the answer type allows) until the
session moves.

6. **A display hint per message type** in `data-entry.yaml`:
`display: cards` with a `cards:` map naming the properties to use (`body`,
`items`, `title`, `plus`, `minus`, `badge`). It shows the body as prose without
a label column, the items as cards with + and − lines and a badge, and hides the
raw items list. Presentation only.

7. **Reply quotes render as plain text.** Strip markdown from the excerpt;
today `**` shows literally.

## Out of scope

- Nested object lists; object types in schema.yaml entities.
- A server check that an answer is one of the options (unchanged).
- Agent-runner changes: it is a consumer of this ticket.

## Example

An agent posts four planning questions as one batch; the first moves the session
to `asking`. The person sees "Question 2 of 4" with two option cards (A with a
"recommended" badge, B), each with + and − lines and a "Choose" button, plus
"Other answer...". Question 1 is collapsed to its answer with a "Change" link.
Each click posts an answer at once. The fourth answer brings
`session.open_questions` to 0, the session moves to `working`, and the agent's
session watch wakes once.

## Out of scope

Crash repair for batches on the file backend. The file backend is the
developer tier, with the project in git; sqlite and postgres write a batch in
one transaction. See `.claude/rules/storage.md`.
