---
id: RR-ED7R4Z5
type: review-response
title: "An ended session at start rendered the app without a schema"
finding: "When the status read answers 401, App.vue returned before the schema load, so the shell rendered under the session dialog with an empty schema store."
severity: significant
resolution: "App.vue skips the load only while a page replaces the app; an ended session loads under its dialog as before."
status: addressed
---
