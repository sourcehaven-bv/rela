---
id: RR-ZYCVZ0
type: review-response
title: Automation and recreate bypass the ref write gate
finding: Automation set:, cascade create_entity properties and RecreateEntity skip the external-ref refusal.
severity: minor
resolution: 'Automation set:, cascade create_entity and recreate refuse refs; automations naming a ref are refused at load; CLI restore keeps refs, data-entry restore does not. Test: TestExternalRef_SystemWritesRefused plus parse rows.'
status: addressed
---
