---
id: BUG-P0DZA5
type: bug
title: Tracked vite.config.js shadows vite.config.ts — fresh-clone dev server breaks
description: 'frontend/vite.config.js and vite.config.d.ts are committed compiler output: tsconfig.node.json sets composite without noEmit, so vue-tsc -b (run by npm run build / build:e2e) emits them next to vite.config.ts. Vite resolves vite.config.js BEFORE vite.config.ts, so the tracked copy wins. The tracked .js predated the __E2E_TEST_HOOKS__ define — on a fresh clone, npm run dev loaded the stale config and MarkdownEditor.vue threw a ReferenceError. The files also silently regenerate on every build, churning into unrelated commits (happened twice during the 2026-06 review work).'
priority: high
effort: s
why1: Vite's config resolution tries vite.config.js before vite.config.ts; a tracked, stale .js predating the __E2E_TEST_HOOKS__ define won, so the dev server ran without the define and the first component referencing it threw.
why2: vue-tsc -b emits .js/.d.ts next to vite.config.ts because tsconfig.node.json declares composite (required for the project reference) without suppressing emit, and the emitted files were committed once and kept being regenerated.
why3: Nothing ignored the emitted filenames — no frontend/.gitignore entry and no root entry — so generated output looked like ordinary source in git status and slipped into commits (plausible-looking generated files pass review).
why4: The node tsconfig was scaffolded for project references before TypeScript supported noEmit in composite projects, and was never revisited when that became available — check-only operation was the intent all along.
why5: No CI step asserts a clean work tree after the build, so build side-effects that mutate the repo are invisible to CI — any generated-file churn lands silently. The systemic fix is the clean-worktree guard, which catches the whole class.
prevention: The CI 'Work tree clean after build' tripwire fails any PR whose frontend job steps modify tracked files or leave untracked files — catching the whole generated-file-churn class, not just these two artifacts. frontend/.gitignore keeps the artifact names as a backstop, and tsconfig.node.json's outDir comment explains why emit is redirected.
status: done
---

Found in the 2026-06-09 frontend architecture review (finding A3, tooling
section). Fix: delete both tracked artifacts; redirect tsconfig.node.json's emit
via `outDir: node_modules/.cache/tsc-node` (plain `noEmit` is rejected with
TS6310 — referenced composite projects may not disable emit); add
frontend/.gitignore entries as a backstop; add a whole-repo clean-worktree
tripwire to the CI frontend job so build-generated files can never silently land
in a PR again.
