---
id: RR-L3TQTC
type: review-response
title: Restore did not check that the snapshot's face is the addressed face
finding: restoreRecreate and the CLI restore trusted the face-scoped reader and did not compare snap.Face with the address.
severity: minor
resolution: Both now compare snap.Face with the addressed face (404 on HTTP, an error on the CLI) as a belt-and-braces check next to the existing type check.
status: addressed
---
