---
id: BUG-E2CITD
type: bug
title: CalDAV ETag of a completed to-do without a visible date changes every second
description: The CalDAV ETag of a completed to-do without a visible date changes every second.
priority: low
status: backlog
---

## Description

A completed CalDAV to-do with no date visible to the principal gets an ETag that
changes every second, so clients re-download it on every sync. Found by the
TKT-0M4H0S golden parity corpus, whose fixture works around it with a
`closed_on` date.

Likely cause: the VTODO serializer stamps a completion time from the clock when
no visible completion date exists, and the ETag hashes the serialized body.

## Fix

Derive the completion timestamp from stored data only (or omit it), so the body
and ETag are stable; pin with a test that serializes the same to-do twice across
a clock change.
