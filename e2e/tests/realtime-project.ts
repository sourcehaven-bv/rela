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
}

/** realtime.yaml for the feature project. */
export function roomRealtimeYaml(opts: RoomOptions = {}): string {
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
      lead:   { permission: channel-lead }
    message_types:
      text:
        label: Text
        properties:
          body: { type: markdown, required: true, max: 2000, label: ${ROOM.messageLabel} }
        post: [member, lead]${contained}
    lifecycle:
      close:  { roles: [lead], label: Close }
      reopen: { roles: [lead], label: Reopen }
    message_ops:
      edit_own:   { roles: [member, lead] }
      delete_own: { roles: [member, lead] }
${session}`;
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
