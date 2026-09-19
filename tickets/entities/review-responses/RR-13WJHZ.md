---
id: RR-13WJHZ
type: review-response
title: 'Review cleanups: AC12 reasoning, unique: list, transport citation, AC14 budget target'
finding: 'Four smaller issues. (1) AC12''s assertion target is right but its reasoning implies incoming edges go through a DIFFERENT gate; since both directions ride one create body, testing both directions exercises one gate with two inputs, and an implementer believing otherwise writes the wrong test. (2) The plan should not imply a unique: LIST property needs an AC — unique.go:69-71 skips list properties outright, so it cannot collide. Separately, loader.go:1157-1162 rejects unique: only on non-string types, so unique+list loads clean and enforces nothing; that is a pre-existing trap worth one sentence since the whole create-form design rests on unique: being real. (3) The transport rule is cited twice as EntityDetail.vue:1504-1518; the actual text is in the startCreate docblock at :809-828. (4) AC14 asserts the detail-page count is unchanged, which is a no-op since the relations fetch is on modal-open; the useful unbudgeted target is GET /{plural}/{id}/relations itself, which scales with relation count.'
severity: minor
resolution: All four applied: AC12 restated as one gate with two inputs; unique:+list caveat added; citation corrected to EntityDetail.vue:809-828; AC19d adds a budget on the relations endpoint.
status: addressed
---

## Resolution

1. Restate AC12 as one gate with two inputs, keeping the explicit non-creatable
verdict requirement (undeclared relation types are default-permissive, so a
verdict-free schema passes vacuously).
2. Add the `unique:` + `list:` caveat as a single sentence under "Why not a
direct server-side copy". Also note enforcement is per-face (`unique.go:82-102`)
and check-then-write on fs/mem.
3. Fix the citation to `EntityDetail.vue:809-828`.
4. Keep AC14 as a regression guard, add a budget on the relations endpoint.
