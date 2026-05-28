---
id: RR-CN7Q
type: review-response
title: 'Cranky #3: NewServer validated only Principal, not Deps fields'
finding: 'NewServer checked only the Principal; a Deps with nil Store/Meta/Watcher/etc. or empty ProjectRoot would defer the failure to request time (nil-deref in a handler, or CWD-walk). CLAUDE.md: ''Constructors reject nil required fields.'' Reviewer noted this was equivalent to the prior interface-nil behavior (not a regression) but the natural place to fix now that fields are enumerable on a struct.'
severity: minor
resolution: 'Added Deps.validate() (required: Store, Meta, Tracer, Searcher, Validator, EntityManager, Config, Watcher, ProjectRoot; LuaCache optional). NewServer calls it after the Principal check. Covered by TestNewServer_RejectsIncompleteDeps in internal/mcp/principal_test.go.'
reason: 'Added Deps.validate() (required: Store, Meta, Tracer, Searcher, Validator, EntityManager, Config, Watcher, ProjectRoot; LuaCache intentionally optional since nil is a valid ''no cache'' signal). NewServer calls it after the Principal gate. Covered by TestNewServer_RejectsIncompleteDeps.'
status: addressed
---
