---
id: RR-KQUI1U
type: review-response
title: i18n keys are split between two catalogues and the app locales are missing
finding: 'Step 7 builds the Other label in channelView.ts with `t(''channels.choiceOther'')` (app i18n, src/i18n/locales/en.json and nl.json, where `channels.decisionStale` lives), while step 9 adds `choiceOther` to rela-components useMessages.ts/messages.nl.ts. Pick one: the view is built in the app, so the label belongs in src/i18n/locales/{en,nl}.json; the group label and list hint belong in the library. Add the locale files to the file list.'
severity: minor
resolution: choiceOther lives in the app locales src/i18n/locales/{en,nl}.json next to decisionStale. The group label and hint live in the library messages, and both locale files are in the file list.
status: addressed
---
