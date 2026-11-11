/**
 * The flows e2e project (TKT-ZAXRXND): a person type, an ACL with an HR user
 * and a one-step onboarding flow, reduced from `internal/flowcfg/testdata`.
 *
 * The flow starts by hand, waits in `collect` for the `hr` role, and moves to
 * the final state `finished` once the step sets `documents`. That is the
 * shortest path from start to a finished run that still crosses every part
 * of the run UI: the Start button, why the run waits, the step form and the
 * finished status.
 *
 * This module holds data and a writer only, and imports nothing from
 * Playwright, like `faced-project.ts`.
 */
import * as fs from "fs";
import * as path from "path";

/** Principals the ACL assigns a role to. `RELA_DATAENTRY_USER` selects one. */
export const FLOW_USERS = {
  /** Holds `hr`: reads and updates persons and may start onboarding. */
  hr: "hr@example.com",
} as const;

/** Seed rows and flow names, in the words the spec asserts on. */
export const FLOW_SEED = {
  person: { id: "PER-1", name: "Ann Smith" },
  flow: { name: "onboarding", label: "Onboarding" },
  /** The state labels the run card shows. */
  states: { collect: "Collect documents", finished: "Finished" },
  role: "hr",
  /** The label of the property the collect step writes. */
  writes: "Documents",
} as const;

export const FLOW_SCHEMA_YAML = `
version: "1.0"
entities:
  person:
    label: Person
    plural: persons
    id_prefix: "PER-"
    id_type: sequential
    properties:
      name:
        type: string
      status:
        type: enum
        values: [new, hired, active, left]
      documents:
        label: ${FLOW_SEED.writes}
        type: boolean
`;

export const FLOW_DATA_ENTRY_YAML = `
version: "1.0"
app:
  name: "Flows E2E"
lists:
  persons:
    entity_type: person
    title: "Persons"
    create_form: person
    edit_form: person
    columns:
      - { property: name, sortable: true, link: detail }
      - { property: status }
forms:
  person:
    entity_type: person
    title: "Person"
    fields:
      - { property: name }
      - { property: status }
      - { property: documents }
navigation:
  - { label: "Persons", list: persons }
`;

export const FLOW_ACL_YAML = `
roles:
  hr:
    read: [person]
    create: [person]
    update: [person]
    permissions: [onboarding.start]
  onboarding-bot:
    read: [person]
    update: [person]
assignments:
  ${FLOW_USERS.hr}: hr
  system:onboarding-bot: onboarding-bot
`;

export const FLOW_FLOWS_YAML = `
flows:
  ${FLOW_SEED.flow.name}:
    label: ${FLOW_SEED.flow.label}
    subject: person
    run_as: system:onboarding-bot
    per_subject: one
    start:
      - on: manual
        permission: onboarding.start
    initial: collect
    states:
      collect:
        label: ${FLOW_SEED.states.collect}
        waiting_for: [${FLOW_SEED.role}]
        writes: [documents]
        on:
          change:
            - to: finished
              roles: [${FLOW_SEED.role}]
              when: "entity.documents == true"
      finished:
        label: ${FLOW_SEED.states.finished}
        final: true
`;

function personFile(id: string, name: string): string {
  return `---\nid: ${id}\ntype: person\nname: ${name}\nstatus: new\ndocuments: false\n---\n`;
}

/** Write the flows project into `dir`, which must exist and be empty. */
export function writeFlowsProject(dir: string): void {
  fs.writeFileSync(path.join(dir, "schema.yaml"), FLOW_SCHEMA_YAML);
  fs.writeFileSync(path.join(dir, "data-entry.yaml"), FLOW_DATA_ENTRY_YAML);
  fs.writeFileSync(path.join(dir, "acl.yaml"), FLOW_ACL_YAML);
  fs.writeFileSync(path.join(dir, "flows.yaml"), FLOW_FLOWS_YAML);
  const file = path.join(dir, "entities", "persons", `${FLOW_SEED.person.id}.md`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, personFile(FLOW_SEED.person.id, FLOW_SEED.person.name));
}
