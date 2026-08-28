---
id: RR-UQ2MIV
type: review-response
title: 'Breaking change breaks the #997 e2e regression guard; blast radius never quantified'
finding: 'The plan''s blast-radius analysis stopped at ''changelog + upgrade notes'' and never grepped the test suite or counted affected configs. `e2e/pages/entity.page.ts:116` asserts `.entity-list .list-item .section-edit-form` is visible — the issue #997 unmount-crash regression guard, called from `e2e/tests/entity-detail-list-unmount.spec.ts:41`. Its fixture (e2e/tests/fixtures.ts:1181-1192) declares a `display: list` section with `- property: status` / `- property: priority` and no `render:`, so after this change the row renders display and the guard fails. The fixture comment at :1164-1169 states the shape exists specifically to mount a SectionEditForm.'
severity: significant
resolution: 'Plan updated: the e2e fixture gains `render: input` on both list-section fields to preserve the #997 guard, and this is now an explicit test-plan item rather than an afterthought. Blast radius quantified: 424 `- property:` entries across three in-repo data-entry.yaml files (tickets/ 271, prototypes/project 107, prototypes/catalog 46).'
status: addressed
---

Verified by grep and by reading the fixture.

The guard:
```ts
// e2e/pages/entity.page.ts:115-117
async expectListSectionRowMounted() {
  await expect(this.page.locator('.entity-list .list-item .section-edit-form').first())
    .toBeVisible();
}
```

The fixture that feeds it (`e2e/tests/fixtures.ts:1186-1192`) has no `render:`,
so both fields default to display after this change and no `.section-edit-form`
mounts.

Fix: add `render: input` to the `Implements` list section's `status` and
`priority` fields so the regression guard keeps exercising the #997 DOM shape.

Blast radius (`- property:` counts, all surfaces):
- `tickets/data-entry.yaml` — 271
- `prototypes/data-entry/project/data-entry.yaml` — 107
- `prototypes/data-entry/catalog/data-entry.yaml` — 46

These lose inline edit until updated. Accepted deliberately by the requester;
the number belongs in the upgrade notes.
