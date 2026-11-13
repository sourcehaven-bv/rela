---
id: RR-LFWLYQT
type: review-response
title: SequentialNumber allocates per row
finding: SequentialNumber upper-cased and concatenated the prefix on every call, once per stored id on the file backends.
severity: nit
resolution: SequentialNumber now trims and compares in place and allocates nothing.
status: addressed
---
