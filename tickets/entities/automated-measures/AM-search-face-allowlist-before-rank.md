---
id: AM-search-face-allowlist-before-rank
type: automated-measure
title: 'Test: free-text search ranks only readable faces'
description: For a type@face principal, free-text search finds an entity whose readable face matches, although a withheld face ranks higher in the world, and list ?q= never matches withheld-face text. To be added with the BUG-OJPVPG fix.
kind: test
location: internal/dataentry search tests + storetest visible-search contract (planned)
status: proposed
---
