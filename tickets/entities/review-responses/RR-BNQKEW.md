---
id: RR-BNQKEW
type: review-response
title: Redundant '..' guard in rootfs
finding: strings.Contains(name, "..") rejects names like notes..yaml that fs.ValidPath allows.
severity: nit
resolution: Kept.
reason: CodeQL needs the guard to see the path is safe; top-level config names are fixed, so no real file is refused.
status: wont-fix
---
