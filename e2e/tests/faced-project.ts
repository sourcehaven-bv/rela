/**
 * The faced e2e project (TKT-WCMW47): a type with two faces, a faceless type
 * related to it, two declared worlds, a transform, an anchored document and an
 * ACL with an all-faces editor and a published-only reader.
 *
 * Kept apart from `fixtures.ts` because that project is faceless on purpose:
 * dozens of specs depend on its shape, and declaring worlds there would change
 * what every one of them reads. This module holds data and a writer only, and
 * imports nothing from Playwright, so it can also be run on its own to
 * materialise the project for manual probing. With a Node that strips types
 * (22.18 or later), from `e2e/`:
 *
 *     node --input-type=module -e "import {writeFacedProject} from './tests/faced-project.ts'; writeFacedProject(process.argv[1])" <dir>
 */
import * as fs from "fs";
import * as path from "path";

/** Principals the ACL assigns a role to. `RELA_DATAENTRY_USER` selects one. */
export const FACED_USERS = {
  /** Reads every face and both worlds; writes `policy@draft`; may publish. */
  editor: "editor@example.com",
  /** Reads `policy@published`, controls, and the published world only. */
  reader: "reader@example.com",
} as const;

export type FacedUser = (typeof FACED_USERS)[keyof typeof FACED_USERS];

/** Declared worlds. `published` is the configured `default_world`. */
export const WORLD = {
  published: "published",
  editorial: "editorial",
} as const;

export const FACE = {
  draft: "draft",
  published: "published",
} as const;

/** Seed rows, in the words the specs assert on. */
export const FACED_SEED = {
  /** Has both a draft and a published face, with different titles and bodies. */
  both: {
    id: "POL-1",
    draftTitle: "Access Control (draft)",
    publishedTitle: "Access Control",
    draftBody: "Draft wording of the access policy.",
    publishedBody: "Published wording of the access policy.",
  },
  /** Has only a draft face, so the published world excludes it. */
  draftOnly: {
    id: "POL-2",
    title: "Retention Schedule",
    body: "Only a draft exists.",
  },
  /** Faceless controls. POL-1@draft implements CTL-1 and POL-1@published
   *  implements CTL-3 (content scope); POL-1 is owned by CTL-2 (identity
   *  scope, shared by both faces). */
  controls: {
    badge: { id: "CTL-1", title: "Badge readers" },
    visitors: { id: "CTL-2", title: "Visitor log" },
    spare: { id: "CTL-3", title: "Clean desk" },
  },
} as const;

export const FACED_SCHEMA_YAML = `
version: "1.0"

entities:
  policy:
    label: Policy
    plural: policies
    id_prefix: POL
    id_type: sequential
    faces:
      draft:     { label: "Draft" }
      published: { label: "Published" }
    properties:
      title:    { type: string, required: true }
      owner:    { type: string }
      evidence: { type: file, max: 3 }

  control:
    label: Control
    id_prefix: CTL
    id_type: sequential
    properties:
      title: { type: string, required: true }

relations:
  # Content scope: the edge belongs to one face.
  implements:
    from: [policy]
    to: [control]
    inverse: implemented-by
    scope: content
  # Identity scope: the edge is shared by every face.
  owned-by:
    from: [policy]
    to: [control]
    inverse: owns
    scope: identity
  # Faceless source, faced target: the shape BUG-BZQQDP reports as 404.
  mitigates:
    from: [control]
    to: [policy]
    inverse: mitigated-by

worlds:
  published:
    select: published
    otherwise: exclude
    banner: "Published view"
  editorial:
    select: [draft, published]
    otherwise: default
    create: draft
    banner: "Editorial view"

copies:
  publish:
    from: policy@draft
    to: policy@published
    label: Publish
    fields: all
    relations:
      implements: replace
    guard:
      permission: publish-policy

# \`cat\` exists on every CI runner; stdin in, stdout out.
transforms:
  markdown:
    command: ["cat"]
    produces: text/markdown

comments:
  enabled: true
  on: ["*"]
`;

export const FACED_DATA_ENTRY_YAML = `
version: "1.0"

app:
  name: "Faces E2E"
  default_world: ${WORLD.published}

lists:
  policies:
    entity_type: policy
    title: "Policies"
    create_form: policy
    create_world: ${WORLD.editorial}
    edit_form: policy
    columns:
      - { property: title, sortable: true, link: detail }
      - { property: owner }
  controls:
    entity_type: control
    title: "Controls"
    create_form: control
    edit_form: control
    columns:
      - { property: title, sortable: true, link: detail }

forms:
  policy:
    entity_type: policy
    title: "Policy"
    body: true
    fields:
      - { property: title }
      - { property: owner }
      - { property: evidence }
    relations:
      - { relation: implements, direction: outgoing, widget: multi-select, label: "Implements" }
      - { relation: owned-by, direction: outgoing, widget: multi-select, label: "Owned by" }
  control:
    entity_type: control
    title: "Control"
    fields:
      - { property: title }
    relations:
      - { relation: mitigates, direction: outgoing, widget: multi-select, label: "Mitigates" }

documents:
  policy-summary:
    title: "Policy summary"
    entity_type: policy
    command: ["cat", "{in}"]

# A detail-page script action is the one web-app route to a FAMILY delete:
# rela.delete_entity on the bare id removes every face (BUG-1YN750).
actions:
  retire:
    label: "Retire"
    script: retire.lua
    available_on:
      entity_types: [policy]

navigation:
  - { label: "Policies", list: policies }
  - { label: "Controls", list: controls }
`;

// The member-of gate is not decoration: acl.yaml refuses to load a policy that
// grants read on a non-default world while the membership relation is ungated.
export const FACED_ACL_YAML = `
role_relations:
  member-of:
    requires_permission: manage-roles

roles:
  editor:
    read: ["*", "world:${WORLD.published}", "world:${WORLD.editorial}"]
    permissions:
      - manage-roles
      - publish-policy
      - history:read
      - comment:read
      - comment:add
      - comment:update-any
      - comment:delete-any
    create: ["*", "policy@draft"]
    update: ["*", "policy@draft"]
    delete: ["*", "policy@draft"]
  reader:
    read: ["policy@published", "control", "world:${WORLD.published}"]

assignments:
  ${FACED_USERS.editor}: editor
  ${FACED_USERS.reader}: reader
`;

export const FACED_RETIRE_LUA = `-- Deletes the whole entity (every face) by its bare id.
rela.delete_entity(entity.id)
return { message = "Retired" }
`;

function policyFile(id: string, title: string, body: string, owner?: string): string {
  const ownerLine = owner ? `owner: ${owner}\n` : "";
  return `---\nid: ${id}\ntype: policy\ntitle: ${title}\n${ownerLine}---\n${body}\n`;
}

function controlFile(id: string, title: string): string {
  return `---\nid: ${id}\ntype: control\ntitle: ${title}\n---\n`;
}

/** A content-scoped edge names its source face in `from_face`; the filename
 *  carries it as `ID@face`. An identity-scoped edge names neither. */
function relationFile(from: string, type: string, to: string, fromFace?: string): string {
  const faceLine = fromFace ? `from_face: ${fromFace}\n` : "";
  return `---\nfrom: ${from}\n${faceLine}relation: ${type}\nto: ${to}\n---\n`;
}

const S = FACED_SEED;

/** Files keyed by project-relative path. A face lives only in the filename
 *  stem (`ID@face`), never in frontmatter. */
const FACED_FILES: Record<string, string> = {
  [`entities/policies/${S.both.id}@draft.md`]: policyFile(
    S.both.id, S.both.draftTitle, S.both.draftBody, "Edith"),
  [`entities/policies/${S.both.id}@published.md`]: policyFile(
    S.both.id, S.both.publishedTitle, S.both.publishedBody, "Edith"),
  [`entities/policies/${S.draftOnly.id}@draft.md`]: policyFile(
    S.draftOnly.id, S.draftOnly.title, S.draftOnly.body),
  [`entities/controls/${S.controls.badge.id}.md`]: controlFile(
    S.controls.badge.id, S.controls.badge.title),
  [`entities/controls/${S.controls.visitors.id}.md`]: controlFile(
    S.controls.visitors.id, S.controls.visitors.title),
  [`entities/controls/${S.controls.spare.id}.md`]: controlFile(
    S.controls.spare.id, S.controls.spare.title),
  [`relations/${S.both.id}@draft--implements--${S.controls.badge.id}.md`]: relationFile(
    S.both.id, "implements", S.controls.badge.id, "draft"),
  [`relations/${S.both.id}@published--implements--${S.controls.spare.id}.md`]: relationFile(
    S.both.id, "implements", S.controls.spare.id, "published"),
  [`relations/${S.both.id}--owned-by--${S.controls.visitors.id}.md`]: relationFile(
    S.both.id, "owned-by", S.controls.visitors.id),
};

/** Write the faced project into `dir`, which must exist and be empty. */
export function writeFacedProject(dir: string): void {
  fs.writeFileSync(path.join(dir, "schema.yaml"), FACED_SCHEMA_YAML);
  fs.writeFileSync(path.join(dir, "data-entry.yaml"), FACED_DATA_ENTRY_YAML);
  fs.writeFileSync(path.join(dir, "acl.yaml"), FACED_ACL_YAML);
  fs.mkdirSync(path.join(dir, "actions"), { recursive: true });
  fs.writeFileSync(path.join(dir, "actions", "retire.lua"), FACED_RETIRE_LUA);
  for (const [rel, content] of Object.entries(FACED_FILES)) {
    const file = path.join(dir, rel);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content);
  }
}
