---
id: RR-WMTTE5
type: review-response
title: Ephemeral line renders raw values and drops refs
finding: 'RlChannel.vue:105-111. ephemeralText joins Object.values(props) in insertion order. Datetimes show as ISO strings in UTC, enums as raw values without the enumOptions label, markdown as source, and refs vanish silently. Message rows get `text`/`enumOptions`/RlRefChip handling and ephemerals get none, so a status signal can read "Status · in_progress · 2026-10-20T09:00:00Z". Fix: give ChannelEphemeralView a preformatted `text` (or `summary`) built by the app, and fall back to the current join only without it. Alternatively, run the values through resolveChannelFields for order and labels.'
severity: minor
resolution: ChannelEphemeralView gained an app-built text, and the fallback now uses presentation order, enum labels and readable ref titles.
status: addressed
---
