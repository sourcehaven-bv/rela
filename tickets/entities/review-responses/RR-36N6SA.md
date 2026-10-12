---
id: RR-36N6SA
type: review-response
title: Rounds silently idle without a replay handler
finding: 'Architecture review: NewRouter installs the replay handler as a side effect. An entry point that starts watching but builds its own router would get streams with an id that never frame, with no error.'
severity: minor
resolution: A round that finds no handler logs an error once (sync.Once). The handler stays installed by NewRouter, which owns the chain it replays.
status: addressed
---
