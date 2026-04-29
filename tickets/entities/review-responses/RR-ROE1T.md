---
id: RR-ROE1T
type: review-response
title: Stale built bundle still has old toLocaleDateString
finding: internal/dataentry/static/v2/assets/format-CmmE9bHn.js contains the old toLocaleDateString() with no options. If rela-server is built without re-running npm run build, the binary ships unfixed format. Confirm just build re-runs npm run build (or rebuild bundle and commit).
severity: significant
resolution: Confirmed internal/dataentry/static/v2/ is gitignored (verified with git check-ignore). The bundle is never committed; CI rebuilds it via just build → build-server → build-frontend → npm run build. Local rebuild with npm run build produced format-ClDt_LxB.js containing year:\"numeric\" and month:\"short\", confirming the new format is in the built artifact. The 'stale' bundle the reviewer saw was a local artifact only.
status: addressed
---
