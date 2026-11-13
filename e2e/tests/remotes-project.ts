/**
 * The remotes e2e project (TKT-0FMYPEZ): one remote `cat` in remotes.yaml,
 * a role that holds remote:sync, and a conflict list seeded in the state
 * directory. The list holds a both-changed field, a superseded entry whose
 * overwritten value was not kept, an uncertain create, and an entry on a row
 * the user cannot read, which the page must not show or count.
 *
 * No remote server runs: the page reads the config and the stored list, and
 * resolve, dismiss and link change only the list. Like `flows-project.ts`,
 * this module imports nothing from Playwright.
 */
import * as fs from "fs";
import * as path from "path";

export const REMOTE_USERS = {
  /** Holds `syncer`: reads and updates requirements, and holds remote:sync. */
  syncer: "syncer@example.com",
} as const;

export const REMOTE_SEED = {
  remote: "cat",
  both: { id: "REQ-1", title: "Local title", remote: "Remote title" },
  superseded: { id: "REQ-2" },
  uncertain: { id: "REQ-3" },
  /** A row of a type the user cannot read. */
  hidden: { id: "SEC-1" },
} as const;

const SCHEMA_YAML = `
version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title: {type: string}
      cat: {type: external_ref, system: catalog}
  secret:
    label: Secret
    plural: secrets
    id_prefix: "SEC-"
    id_type: sequential
    properties:
      title: {type: string}
      cat: {type: external_ref, system: catalog-secret}
`;

const DATA_ENTRY_YAML = `
version: "1.0"
app:
  name: "Remotes E2E"
lists:
  requirements:
    entity_type: requirement
    title: "Requirements"
    columns:
      - { property: title, link: detail }
navigation:
  - { label: "Requirements", list: requirements }
`;

const ACL_YAML = `
roles:
  syncer:
    read: [requirement]
    update: [requirement]
    permissions: [remote:sync]
assignments:
  ${REMOTE_USERS.syncer}: syncer
`;

const REMOTES_YAML = `
remotes:
  ${REMOTE_SEED.remote}:
    url: https://catalog.example
    credential: {env: REMOTE_CREDENTIAL_CAT}
    superseded_retention: "0"
    entities:
      requirement:
        ref: cat
        fields: {title: title}
`;

const SINCE = "2026-11-11T10:00:00Z";

function conflicts(): string {
  const base = { type: "requirement", rule: "latest", since: SINCE, seen: SINCE };
  return JSON.stringify({
    version: 1,
    conflicts: [
      {
        ...base,
        local: REMOTE_SEED.both.id,
        address: "requirement/R-1",
        field: "title",
        remote_field: "title",
        reason: "both_changed",
        fallback: "same_time",
        remote_value: REMOTE_SEED.both.remote,
      },
      {
        ...base,
        local: REMOTE_SEED.superseded.id,
        address: "requirement/R-2",
        field: "title",
        remote_field: "title",
        reason: "superseded",
        remote_value: "Their winning title",
        superseded: {
          winner: "remote",
          local_updated: "2026-11-11T09:00:00Z",
          remote_updated: SINCE,
        },
      },
      {
        ...base,
        local: REMOTE_SEED.uncertain.id,
        reason: "uncertain",
        cause: "no_answer",
        refused_fields: { "title<title": "d" },
      },
      {
        ...base,
        type: "secret",
        local: REMOTE_SEED.hidden.id,
        address: "secret/S-1",
        field: "title",
        remote_field: "title",
        reason: "both_changed",
        remote_value: "classified",
      },
    ],
  });
}

function entity(dir: string, plural: string, type: string, id: string, title: string) {
  const file = path.join(dir, "entities", plural, `${id}.md`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `---\nid: ${id}\ntype: ${type}\ntitle: ${title}\n---\n`);
}

/** Write the remotes project into `dir`, which must exist and be empty. */
export function writeRemotesProject(dir: string): void {
  fs.writeFileSync(path.join(dir, "schema.yaml"), SCHEMA_YAML);
  fs.writeFileSync(path.join(dir, "data-entry.yaml"), DATA_ENTRY_YAML);
  fs.writeFileSync(path.join(dir, "acl.yaml"), ACL_YAML);
  fs.writeFileSync(path.join(dir, "remotes.yaml"), REMOTES_YAML);
  entity(dir, "requirements", "requirement", REMOTE_SEED.both.id, REMOTE_SEED.both.title);
  entity(dir, "requirements", "requirement", REMOTE_SEED.superseded.id, "Second");
  entity(dir, "requirements", "requirement", REMOTE_SEED.uncertain.id, "Third");
  entity(dir, "secrets", "secret", REMOTE_SEED.hidden.id, "Hidden");
  const state = path.join(dir, ".rela", "remotesync", REMOTE_SEED.remote);
  fs.mkdirSync(state, { recursive: true });
  fs.writeFileSync(path.join(state, "conflicts.json"), conflicts());
}
