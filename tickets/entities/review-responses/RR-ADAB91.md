---
id: RR-ADAB91
type: review-response
title: 'Design: store reads the principal from ctx'
finding: The store may learn the principal only through boundary-filled inputs.
severity: significant
resolution: TagRequest carries PrincipalUser/Tool from the entitymanager; zero principal refused (R3).
status: addressed
---
