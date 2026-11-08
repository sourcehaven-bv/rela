---
id: RR-E4ATSX0
type: review-response
title: Project containment check ignores symlinks
finding: inside() compared literal paths, so /var and /private/var or a symlinked TMPDIR could evade it.
severity: minor
resolution: inside() resolves symlinks in both paths first; TestInside covers a symlinked spelling. The godoc notes NewInMemory fails when TMPDIR is inside the project.
status: addressed
---
