/**
 * realtime.yaml and acl.yaml for the channel specs (TKT-91BK6F).
 *
 * The shared fixture project has neither file, so realtime is off for every
 * other spec. A channel spec writes them into its own temp project before
 * the server starts, and rewrites realtime.yaml while it runs: the server
 * watches that file and applies a change without a restart. acl.yaml is
 * read once at start.
 *
 * No Playwright import, like faced-project.ts, so the files can be written
 * by hand for manual probing.
 */
import * as fs from "fs";
import * as path from "path";

/** The principals of the feature spec, sent in the proxy header. */
export const CHANNEL_USERS = {
  /** Posts, and closes and reopens the channel. */
  lead: "alice@example.com",
  /** Posts. */
  member: "bob@example.com",
  /** Reads the entity and the channel, but may not post. */
  watcher: "carol@example.com",
} as const;

/** The channel type on `feature`, and the labels the web app shows. */
export const ROOM = {
  type: "room",
  label: "Discussion",
  messageLabel: "Message",
} as const;

/** The session on ROOM when RoomOptions.session is set. */
export const SESSION = {
  label: "Incident response",
  from: "Detected",
  to: "Contained",
  /** The message type that moves the session; only the lead may post it. */
  messageLabel: "Contained",
  noteLabel: "Note",
  /** A message type with in: [contained]; offered only once the session is contained. */
  wrapUpLabel: "Wrap-up",
} as const;

/**
 * The agent-run session on ROOM when RoomOptions.run is set (TKT-21G6JS).
 * It starts in Queued, which waits for the feature's owner, so a new
 * channel is not busy. Start moves it to Planning, whose status is
 * working, and Finish ends it.
 */
export const RUN = {
  label: "Agent run",
  queued: "Queued",
  planning: "Planning",
  done: "Plan ready",
  startLabel: "Start",
  finishLabel: "Finish",
  noteLabel: "Note",
} as const;

/**
 * The review session on ROOM when RoomOptions.decisions is set (TKT-T9G1AE):
 * a Contained message sends the work to review, where the lead approves it
 * with one click or asks for a revision through the form.
 */
export const DECISIONS = {
  drafting: "Drafting",
  review: "Review",
  approved: "Approved",
  /** Moves Drafting to Review; anyone who posts may send it, only the lead's moves. */
  containedLabel: "Contained",
  noteLabel: "Note",
  /** One-click: no required property, no guard. Moves Review to Approved (final). */
  approveLabel: "Approve",
  /** Opens the form: a required Feedback. Moves Review back to Drafting. */
  reviseLabel: "Revise",
  feedbackLabel: "Feedback",
} as const;

/**
 * The question and answer types on ROOM when RoomOptions.choices is set
 * (TKT-03FQPD). A member asks with a list of options; the composer cannot
 * enter a list, so the spec posts the question over the API. Only the lead
 * may answer, so only the lead sees the reply buttons.
 */
export const CHOICES = {
  askType: "ask",
  questionLabel: "Question",
  optionsProperty: "options",
  answerLabel: "Answer",
  other: "Other answer…",
} as const;

/** The face-anchored channel type on the faced project's `policy`. */
export const POLICY_ROOM = {
  type: "policy-room",
  label: "Policy discussion",
} as const;

export interface RoomOptions {
  /**
   * Narrows who receives the channel to the lead. Without it, everyone who
   * can read the entity receives it.
   */
  leadReadsOnly?: boolean;
  /**
   * Gives the room a session, Detected to Contained. Members and the lead
   * may post a Contained message; only the lead's moves the session. A
   * Wrap-up message may be posted only in Contained.
   */
  session?: boolean;
  /**
   * Gives the room the review session of DECISIONS instead. Cannot be
   * combined with `session`.
   */
  decisions?: boolean;
  /** Adds the question and answer types of CHOICES. No session. */
  choices?: boolean;
  /**
   * Gives the room the agent-run session of RUN instead, and an owner role
   * from the feature's `owner` property, which addBusyBoard adds to the
   * schema. Cannot be combined with `session` or `decisions`.
   */
  run?: boolean;
}

/** realtime.yaml for the feature project. */
export function roomRealtimeYaml(opts: RoomOptions = {}): string {
  if ([opts.session, opts.decisions, opts.run].filter(Boolean).length > 1) {
    throw new Error("RoomOptions: session, decisions and run are exclusive");
  }
  const read = opts.leadReadsOnly ? "{ permission: channel-lead }" : "{ from: attached }";
  const contained = opts.session
    ? `
      contained:
        label: ${SESSION.messageLabel}
        properties:
          note: { type: string, required: true, label: ${SESSION.noteLabel} }
        post: [member, lead]
      wrap-up:
        label: ${SESSION.wrapUpLabel}
        properties:
          note: { type: string, label: ${SESSION.noteLabel} }
        post: [member, lead]
        in: [contained]`
    : "";
  const decisionTypes = opts.decisions
    ? `
      contained:
        label: ${DECISIONS.containedLabel}
        properties:
          note: { type: string, required: true, label: ${DECISIONS.noteLabel} }
        post: [member, lead]
      approve:
        label: ${DECISIONS.approveLabel}
        properties:
          note: { type: string, label: ${DECISIONS.noteLabel} }
        post: [member, lead]
        in: [review]
      revise:
        label: ${DECISIONS.reviseLabel}
        properties:
          feedback: { type: string, required: true, label: ${DECISIONS.feedbackLabel} }
        post: [member, lead]
        in: [review]`
    : "";
  const choiceTypes = opts.choices
    ? `
      ${CHOICES.askType}:
        label: ${CHOICES.questionLabel}
        properties:
          question: { type: string, required: true, label: ${CHOICES.questionLabel} }
          ${CHOICES.optionsProperty}: { type: string, list: true, max: 80, max_items: 4, label: Options }
        post: [member, lead]
        choices: { from: ${CHOICES.optionsProperty}, answer: answer, into: note, other: true }
      answer:
        label: ${CHOICES.answerLabel}
        properties:
          note: { type: string, required: true, label: ${CHOICES.answerLabel} }
        post: [lead]`
    : "";
  const decisionSession = opts.decisions
    ? `    session:
      label: Review
      initial: drafting
      states:
        drafting:
          label: ${DECISIONS.drafting}
          on:
            contained: { to: review, roles: [lead] }
        review:
          label: ${DECISIONS.review}
          on:
            approve: { to: approved, roles: [lead] }
            revise: { to: drafting, roles: [lead] }
        approved: { label: ${DECISIONS.approved}, final: true, status: completed }
`
    : "";
  const runTypes = opts.run
    ? `
      start:
        label: ${RUN.startLabel}
        properties:
          note: { type: string, required: true, label: ${RUN.noteLabel} }
        post: [member, lead, owner]
      finish:
        label: ${RUN.finishLabel}
        properties:
          note: { type: string, required: true, label: ${RUN.noteLabel} }
        post: [member, lead]`
    : "";
  const runSession = opts.run
    ? `    session:
      label: ${RUN.label}
      initial: queued
      states:
        queued:
          label: ${RUN.queued}
          waiting_for: [owner]
          on:
            start: { to: planning, roles: [lead, owner] }
        planning:
          label: ${RUN.planning}
          status: working
          on:
            finish: { to: done, roles: [lead] }
        done: { label: ${RUN.done}, final: true }
`
    : "";
  const ownerRole = opts.run ? "\n      owner:  { from: entity.owner }" : "";
  const session = opts.session
    ? `    session:
      label: ${SESSION.label}
      initial: detected
      states:
        detected:
          label: ${SESSION.from}
          on:
            contained: { to: contained, roles: [lead] }
        contained: { label: ${SESSION.to}, final: true }
`
    : "";
  return `version: 1
channel_types:
  ${ROOM.type}:
    label: ${ROOM.label}
    attach_to: [feature]
    cardinality: one
    typing: true
    read: ${read}
    roles:
      member: { permission: channel-post }
      lead:   { permission: channel-lead }${ownerRole}
    message_types:
      text:
        label: Text
        properties:
          body: { type: markdown, required: true, max: 2000, label: ${ROOM.messageLabel} }
        post: [member, lead]${contained}${decisionTypes}${choiceTypes}${runTypes}
    lifecycle:
      close:  { roles: [lead], label: Close }
      reopen: { roles: [lead], label: Reopen }
    message_ops:
      edit_own:   { roles: [member, lead] }
      delete_own: { roles: [member, lead] }
${session}${decisionSession}${runSession}`;
}

/** acl.yaml for the feature project: everyone reads, two may post. */
export const ROOM_ACL_YAML = `roles:
  lead:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    permissions: [channel-post, channel-lead]
  member:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    permissions: [channel-post]
  watcher:
    read: ["*"]

assignments:
  ${CHANNEL_USERS.lead}: lead
  ${CHANNEL_USERS.member}: member
  ${CHANNEL_USERS.watcher}: watcher
`;

/** realtime.yaml for the faced project: one channel per policy face. */
export const POLICY_REALTIME_YAML = `version: 1
channel_types:
  ${POLICY_ROOM.type}:
    label: ${POLICY_ROOM.label}
    attach_to: [policy]
    anchor: face
    cardinality: one
    roles:
      reader: { from: attached }
    message_types:
      text:
        label: Text
        properties:
          body: { type: markdown, required: true, max: 2000, label: Message }
        post: [reader]
`;

/**
 * The board of the busy spec (TKT-21G6JS): features with a busy field for
 * ROOM. It is added to this project only, because the shared fixture has no
 * realtime.yaml and a busy field names a channel type. addBusyBoard also
 * gives features the `owner` property RUN's waiting role reads.
 */
export const BUSY_BOARD = {
  id: "feature-busy",
  title: "Feature Busy",
} as const;

/**
 * Adds BUSY_BOARD to the data-entry.yaml written by the fixture, and an
 * `owner` string property to `feature` in its schema.yaml.
 */
export function addBusyBoard(dir: string): void {
  const schemaFile = path.join(dir, "schema.yaml");
  const schema = fs.readFileSync(schemaFile, "utf8");
  const feature = /^ {2}feature:\n(?: {4}.*\n)*? {4}properties:\n/m;
  if (!feature.test(schema)) throw new Error("addBusyBoard: schema.yaml has no feature properties");
  writeAtomic(schemaFile, schema.replace(feature, (m) => `${m}      owner:\n        type: string\n`));

  const file = path.join(dir, "data-entry.yaml");
  const yaml = fs.readFileSync(file, "utf8");
  const board = `kanbans:
  ${BUSY_BOARD.id}:
    entity_type: feature
    title: "${BUSY_BOARD.title}"
    column_property: status
    columns:
      - value: draft
        label: Draft
      - value: approved
        label: Approved
    card:
      title: title
      fields:
        - busy: ${ROOM.type}
`;
  if (!/^kanbans:\n/m.test(yaml)) throw new Error("addBusyBoard: data-entry.yaml has no kanbans block");
  writeAtomic(file, yaml.replace(/^kanbans:\n/m, board));
}

/** Writes the feature project's realtime.yaml and acl.yaml into dir. */
export function writeRoomProject(dir: string, opts: RoomOptions = {}): void {
  writeRoomRealtime(dir, opts);
  fs.writeFileSync(path.join(dir, "acl.yaml"), ROOM_ACL_YAML);
}

/** Rewrites realtime.yaml; a running server applies it. */
export function writeRoomRealtime(dir: string, opts: RoomOptions = {}): void {
  writeAtomic(path.join(dir, "realtime.yaml"), roomRealtimeYaml(opts));
}

/** Adds realtime.yaml to a faced project written by writeFacedProject. */
export function writePolicyRealtime(dir: string): void {
  writeAtomic(path.join(dir, "realtime.yaml"), POLICY_REALTIME_YAML);
}

// A rename, so the watcher never reads a half-written file.
function writeAtomic(file: string, content: string): void {
  const tmp = `${file}.tmp`;
  fs.writeFileSync(tmp, content);
  fs.renameSync(tmp, file);
}
