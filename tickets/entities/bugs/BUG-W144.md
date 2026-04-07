---
id: BUG-W144
type: bug
title: Desktop binary ships without Vue SPA assets
description: rela-desktop showed a directory listing instead of the SPA after picking a project, because the embedded static/v2 directory was missing index.html.
priority: high
why1: The desktop binary embeds internal/dataentry/static/v2/ via go:embed, but the directory was empty (or only contained leftover v1 files), so the asset server fell through to a directory listing after a project was loaded.
why2: The Vue SPA was never built before compiling the desktop binary — the build-desktop just recipe (and the desktop CI release job) did not depend on build-frontend.
why3: When the desktop build target was added, it was patterned after build-cli (no frontend) instead of build-server (which already depends on build-frontend), so the embed dependency was silently omitted.
why4: There is no compile-time check that //go:embed targets are non-empty or contain expected files, and no integration test that exercises a packaged desktop binary against a real project.
why5: Embedded asset pipelines have an implicit dependency from Go code on a sibling build system (npm/vite), and our build orchestration does not encode that dependency consistently across all targets that share the same embed.
prevention: Any go:embed of a generated directory must be matched by an explicit build-system dependency on the generator. When adding a new build target that imports a package using //go:embed of generated assets, audit existing recipes that embed the same path and copy their generator dependencies. Consider a smoke test that packages and launches the desktop binary against a fixture project.
status: done
---
