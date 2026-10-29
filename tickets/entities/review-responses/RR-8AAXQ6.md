---
id: RR-8AAXQ6
type: review-response
title: Index export is unnecessary
finding: Plan step 4 exports foldMessages from the package index 'for the app-side test'. The app already deep-imports library modules (channelView.ts:59 imports rela-components/components/channel/channelFields), and the library has only story tests, so the app test can do the same. Drop the index export, or give it a real consumer.
severity: nit
resolution: Dropped the index export; channelFold.test.ts deep-imports the module as channelView.ts does with channelFields.
status: addressed
---
