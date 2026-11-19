import fs from 'node:fs';
import path from 'node:path';
import { test, SEED } from './fixtures';
import { EntityPage } from '../pages';

// TKT-14FBF1R: an entity page leaves out a section the reader gets nothing
// from. Two cases on the fixture's task view:
//   "Classification"  every field (kind, areas) is hidden by `visible:`
//   "Staff notes"     gated by `permission: staff.manage`
// The API side is pinned in Go (TestWithheldSections_PerReader); this spec
// proves the page renders no empty heading for either.

const STAFF_SECTION = `      - heading: "Staff notes"
        source: entry
        display: properties
        permission: staff.manage
        fields:
          - property: note
      - heading: "Classification"`;

// A customer reads every task but sees only these fields, and holds no
// permission. Staff have no `visible:` block, so they see every field.
const ACL = (role: 'customer' | 'staff') => `roles:
  customer:
    read: ["*"]
    visible:
      task:
        - field: title
        - field: status
        - field: assignee
        - field: done
        - field: note
  staff:
    read: ["*"]
    permissions: [staff.manage]
assignments:
  e2e@example.com: ${role}
`;

function withPolicy(role: 'customer' | 'staff') {
  return test.extend({
    testProject: async ({ testProject }, use) => {
      const cfgPath = path.join(testProject, 'data-entry.yaml');
      const cfg = fs.readFileSync(cfgPath, 'utf8');
      const gated = cfg.replace('      - heading: "Classification"', STAFF_SECTION);
      if (gated === cfg) throw new Error('fixture task view has no Classification section');
      fs.writeFileSync(cfgPath, gated);
      fs.writeFileSync(path.join(testProject, 'acl.yaml'), ACL(role));
      await use(testProject);
    },
  });
}

withPolicy('customer')('a customer gets no gated section and no section of hidden fields', async ({ appPage }) => {
  const entity = new EntityPage(appPage);
  await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

  await entity.expectSectionShown('Task');
  await entity.expectSectionAbsent('Classification');
  await entity.expectSectionAbsent('Staff notes');
});

withPolicy('staff')('staff get both sections', async ({ appPage }) => {
  const entity = new EntityPage(appPage);
  await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

  await entity.expectSectionShown('Task');
  await entity.expectSectionShown('Classification');
  await entity.expectSectionShown('Staff notes');
});
