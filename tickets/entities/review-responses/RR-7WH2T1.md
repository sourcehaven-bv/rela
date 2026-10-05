---
id: RR-7WH2T1
type: review-response
title: '[security] storedFacesOf read error fails open; purge RecordID not bound to its tail'
finding: 'Security review: duplicates of the cranky storedFacesOf and recordIDIsHeadOfKey findings, rated minor from the security side.'
severity: minor
resolution: 'Fixed with those findings: loadStoredFaces returns the error; recordIDIsHeadOfKey filters from_face.'
status: addressed
---
