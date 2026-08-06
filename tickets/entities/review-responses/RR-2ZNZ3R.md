---
id: RR-2ZNZ3R
type: review-response
title: Guard couples to the vite 'index-' default; a rename causes a false FAIL that invites loosening the regex
finding: 'ASSET_RE hardcodes `assets/index-<hash>`, which comes from rollup naming the entry chunk after index.html — a default, not a contract. It changes under build.rollupOptions.output.entryFileNames, a renamed entry, or build.assetsDir. This repo already customizes output naming in vite.editor.config.ts (fileName -> rela-editor.js), proving the team does this. Failure mode is a false FAIL (safe) but the likely ''fix'' under release pressure is loosening the regex until it matches embed error strings again, reintroducing the bug. Stronger: parse the actual hashed asset name out of the built index.html and grep the binary for THAT name — self-correcting, and proves the binary embeds THIS build rather than some past one.'
severity: significant
resolution: 'Removed the hardcoded `index-` pattern entirely. Both checks now derive the expected asset names from the built index.html (entry_assets()), so a vite entryFileNames/assetsDir rename cannot cause a false FAIL and there is no pattern left to loosen under release pressure. This is also strictly stronger than the original: the binary check now proves the artifact embeds THIS build''s fingerprinted assets, not merely some past build. Pinned by the ''binary embedding only a stale build''s assets fails'' test. Verified live: 37/37 entry assets matched on a good binary, 0/37 on the real broken v0.14.'
status: addressed
---
