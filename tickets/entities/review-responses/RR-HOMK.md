---
id: RR-HOMK
type: review-response
title: 'Stale built asset SettingsView-BibFBYmf.js still committed with old `dark: auto` parser'
finding: |-
    internal/dataentry/static/v2/assets/SettingsView-BibFBYmf.js (committed binary build artifact) still contains the legacy parser code recognising `dark: auto` and `dark: false`. If the server is launched without a fresh frontend build, users hit the old JS bundle. This is the same kind of stale-build foot-gun that bit the original ticket — the v2 directory is checked in, so it can drift from the source.

    Verify: `npm run build` is part of `just build`, but the prebuilt bundle hash will change. If the PR is reviewed without a frontend rebuild, the committed asset is from `develop`, not the new branch. Make sure the PR includes (or CI enforces) a fresh build of `internal/dataentry/static/v2/`.

    Long-term: don't commit built assets. Embed via Go's embed.FS at build time and gitignore the `assets/` subtree. That removes a category of 'works locally, breaks in CI' bugs.
severity: minor
reason: 'False alarm — the cranky reviewer was looking at a stale build artifact (`SettingsView-BibFBYmf.js`) from a previous local build. The path `internal/dataentry/static/v2/` is gitignored (verified `git ls-files internal/dataentry/static/v2/ | wc -l` returns 0). No built assets are tracked in git on this repo. The current build produces `SettingsView-aeaLhgfa.js` (different content hash), and `git status` confirms no static-asset changes are staged. The ''long-term: stop committing built assets'' suggestion is moot — they were never committed.'
status: wont-fix
---
