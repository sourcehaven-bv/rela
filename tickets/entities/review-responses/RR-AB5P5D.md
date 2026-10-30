---
id: RR-AB5P5D
type: review-response
title: Hand-built key path duplicates messageTypeKey
finding: 'internal/realtimecfg/buttonoptions.go:155 builds ct.messageTypesKey() + "." + mn. ct.messageTypeKey(mn) exists for this (message_in.go:10). Fix: key := ct.messageTypeKey(mn) + ".decision_button".'
severity: nit
resolution: key := ct.messageTypeKey(mn) + ".decision_button".
status: addressed
---
