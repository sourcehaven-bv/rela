---
id: RR-4W2ZMK
type: review-response
title: Discovery failure reported at the wrong step
finding: 'restish api connect exits 0 when no spec is found, so a discovery failure surfaced later as ''unknown flag: --filename''.'
severity: significant
resolution: The test runs restish rela --help after connect and fails with 'discovery found no spec at <origin>' when put-ticket-attachment is missing. Mutation check shows the new message.
status: addressed
---
