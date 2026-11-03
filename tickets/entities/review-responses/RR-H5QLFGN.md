---
id: RR-H5QLFGN
type: review-response
title: Convert dialog drops file-property picks
finding: File properties render a FileWidget in create-staging mode. The modal never listens for update:staged-files and never uploads, so a required file property can never be satisfied.
severity: significant
resolution: conversionFields lists file properties apart; the modal says they can be set after the conversion.
status: addressed
---
