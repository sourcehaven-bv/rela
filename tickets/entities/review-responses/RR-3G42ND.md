---
id: RR-3G42ND
type: review-response
title: AbsPath downgrades ValidatedPath to string by convention
finding: The one deliberate string escape. Its single caller only compares paths and never does I/O.
severity: minor
reason: Exporting ValidatedPath accessors is worse. The caller (fsstore layout) is verified I/O-free and the doc comment already forbids passing it back into raw FS methods. Left as documented.
status: wont-fix
---
