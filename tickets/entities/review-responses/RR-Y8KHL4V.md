---
id: RR-Y8KHL4V
type: review-response
title: "Cancelled first sidebar request leaves the sidebar empty"
finding: "When the current sidebar request was cancelled, navLoaded stayed false and no nav rows showed."
severity: minor
resolution: "Addressed: A cancelled current request ends the wait too, so the built-in rows show."
status: addressed
---
