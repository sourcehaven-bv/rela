---
id: RR-S16QPH
type: review-response
title: Test fixture mutates shared config in place
finding: document_face_test assigns app.State().Cfg.Documents directly instead of publishing a new Schema.
severity: nit
resolution: The fixture publishes a new Schema with the documents and the copy transform instead of mutating the live config.
status: addressed
---
