---
id: RR-FB1K
type: review-response
title: 'S7: 401/403 detection by error-message substring is fragile'
finding: |
  `useAutoSave.onError` receives only `info.message` — the status code is parsed inside `parseError` but never passed out. SectionEditForm would have to substring-match server-supplied error text to detect 403s. Fragile.
severity: significant
status: addressed
resolution: |
  Extend `useAutoSave.AutoSaveOptions.onError` to receive structured info: `onError: (msg: string, info?: { status?: number; property?: string; channel?: 'property' | 'content' | 'relations' }) => void`. This is a small back-compat-safe extension (info is optional; existing callers like DynamicForm ignore it). SectionEditForm then dispatches on `info.status === 403` cleanly.

  This is a tiny `useAutoSave` API change scoped to this ticket. The change is purely additive; no existing callers break. Tests in `useAutoSave.test.ts` add a "onError receives status on 403" case.
---
