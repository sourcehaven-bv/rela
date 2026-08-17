---
id: RR-28QDBC
type: review-response
title: 'EntityList.test.ts still leaked real HTTP so the bug was not fully fixed'
finding: 'With the SidePanel stub applied the full suite still issued 21 stray ECONNREFUSED requests - 15 from EntityList.test.ts, which never stubs ExportMenu. ExportMenu fetches in onMounted and logs via console.error (ExportMenu.vue:19-27), the same shape as SidePanel, and api/transforms.ts nulls its cache on rejection so every remount retries. Fixing only the file named in the traceback left the class of failure live.'
severity: significant
resolution: 'Addressed by the global guard in src/test/setup.ts rather than by adding an ExportMenu stub: neither EntityList test file has a stubs block, and the guard covers them without per-file surgery. Verified: src/components/lists/ now runs 57 tests with zero stray requests.'
status: addressed
---
