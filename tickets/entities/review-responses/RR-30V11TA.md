---
id: RR-30V11TA
type: review-response
title: Misplaced flag starts the server instead of erasing
finding: rela-server -project X tenant erase ORG starts the server because flag.Parse ignores trailing args.
severity: minor
resolution: parseFlags refuses leftover arguments via refuseExtraArgs, pointing a misplaced tenant command at the right form. Pinned by TestRefuseExtraArgs.
status: addressed
---
