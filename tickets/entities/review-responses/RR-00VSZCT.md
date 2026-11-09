---
id: RR-00VSZCT
type: review-response
title: Leftover peer-key config loads without warning
finding: 'A develop build after 02ff7340a may hold peer: grants in acl.yaml and peer_key./peer_credential. secrets, which now load silently and are script readable.'
severity: minor
reason: '02ff7340a is in no release tag, so no deployment holds such config; DEC-QKLMMD4 records that peer: has no meaning. A load warning for a name that was never released is not worth the code.'
status: wont-fix
---
