---
id: RR-9UK1YT
type: review-response
title: i18n key name and wording
finding: decisionConfirmMove breaks the pattern of its neighbours (decisionConfirmBody, decisionDiscardEditBody, frontend/src/i18n/locales/en.json:167-173); name it decisionConfirmMoveBody. The body "This moves the session to {state}." is fine; give the Dutch text in the plan ("Hiermee gaat de sessie naar {state}.") so the reviewer can check it, since the message-file test in src/i18n/guards.test.ts only checks parity and usage.
severity: nit
resolution: Renamed the key to channels.decisionConfirmMoveBody and gave the Dutch text "Hiermee gaat de sessie naar {state}." in AC 8.
status: addressed
---
