---
id: RR-C47YN9
type: review-response
title: '[security] Restoring a deleted face can become a whole-record update of a live row'
finding: 'Security review: check-then-act between resolveHistorySubject and ApplyEntity; a concurrent recreate turns the restore into an ungated whole-record replace that erases fields the caller cannot read and skips automations.'
severity: significant
resolution: 'Same fix as the critical cranky finding: create-only entitymanager.RecreateEntity; 409 on a raced recreate; CLI restore uses it too.'
status: addressed
---
