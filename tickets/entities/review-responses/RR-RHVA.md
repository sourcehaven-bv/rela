---
id: RR-RHVA
type: review-response
title: 'N5: validateCacheFilename missed control bytes other than NUL and Windows drive letters'
finding: 'Names like `foo\x01bar.yaml` passed validation despite containing a control character. Names like `c:file.yaml` would be silently misinterpreted on Windows as referring to the c: drive.'
severity: minor
resolution: 'Replaced the NUL-only check with a full control-byte sweep (`r < 0x20 || r == 0x7f`). Added a drive-letter check (reject if name[1] == '':''). Test cases added: `with\x01ctrl.yaml` and `c:file.yaml`.'
status: addressed
---
