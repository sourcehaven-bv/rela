# Codebase Concerns

**Analysis Date:** 2026-03-19

## Tech Debt

**Incomplete Attachment Deduplication Logic:**
- Issue: `AttachResult.Deduplicated` is always set to `false` even when the attachment store performs deduplication
- Files: `internal/workspace/attachment.go:90`
- Impact: Clients cannot distinguish between newly uploaded and previously existing attachments, limiting UI feedback options
- Fix approach: Query the attachment store's deduplication status and populate the result field correctly

**Command Palette Not Implemented:**
- Issue: Frontend keyboard shortcut handler has placeholder comment for command palette feature
- Files: `frontend/src/composables/useKeyboardShortcuts.ts:27`
- Impact: Command palette shortcut binding is non-functional; feature is incomplete
- Fix approach: Implement command palette UI component and integrate with keyboard handler

**Panics in Router Initialization:**
- Issue: Router panics on embedded filesystem errors during template/asset loading
- Files: `internal/dataentry/templates.go:28`, `internal/dataentry/router.go:17,24`
- Impact: Unrecoverable fatal errors if build-time embedding fails; poor error recovery
- Fix approach: Return errors from NewRouter instead of panicking; let caller decide error handling strategy

## Known Bugs

**Potential Deadlock Risk in Live Reload:**
- Symptoms: Multiple concurrent reload operations may contend for write lock
- Files: `internal/dataentry/app.go:55-59` (RWMutex protecting reloadable state)
- Trigger: Simultaneous file change notifications + manual reload + handler requests
- Current mitigation: File watcher has 200ms debounce to batch events
- Workaround: Debounce prevents rapid-fire reloads, reducing contention
- Risk: Under high event frequency or slow handlers, contention is still possible

**AllNodes() O(n) Iteration Called Frequently:**
- Symptoms: List views, search operations, and analysis functions iterate all nodes repeatedly
- Files: `internal/dataentry/handlers_api.go:167`, `internal/dataentry/analyze.go:121,336,394`, `internal/dataentry/helpers.go:343`, `internal/workspace/attachment.go:223`, `internal/graph/graph_perf_test.go:118,130`
- Trigger: Any operation filtering/searching across all entities
- Cause: Graph lacks indexed query support; callers resort to full iteration
- Workaround: 200-node test graphs show acceptable performance; real-world impact depends on project size
- Risk: Performance degrades linearly with entity count; search on large projects (1000+ entities) may be slow

## Security Considerations

**No Input Sanitization in Write Operations:**
- Risk: Entity properties are written directly to markdown/YAML without escaping
- Files: `internal/markdown/writer.go` (implied - entity mutation flows through workspace)
- Current mitigation: Properties are stored as YAML and escaped by yaml.v3
- Recommendations:
  1. Validate that YAML encoding doesn't produce unsafe output (yaml.v3 is safe by design)
  2. Add integration tests verifying unsafe characters are properly escaped in markdown files
  3. Review relation names and property values that flow through user input paths

**Potential XSS in HTML Rendering:**
- Risk: Entity content and descriptions are rendered as HTML templates
- Files: `internal/dataentry/handlers.go` (template execution), handler functions that build HTML
- Current mitigation: Go's `html/template` package auto-escapes by default
- Recommendations:
  1. Ensure all template functions that render user content use safe escaping (not `htmltemplate.HTML` bypass)
  2. Test with malicious HTML in entity descriptions and content fields
  3. Review any custom template functions that may bypass escaping

**No Access Control in API:**
- Risk: API v1 endpoints are completely open; anyone with network access can read/write
- Files: `internal/dataentry/api_v1.go`, `internal/dataentry/handlers_api.go`
- Current mitigation: None (expected in local/network-trusted environments)
- Recommendations:
  1. Add authentication middleware (API key or bearer token)
  2. Add role-based access control for read/write operations
  3. Document the security posture clearly (local-only assumption)

**Git Operations Without Verification:**
- Risk: Git commands executed with user-provided paths
- Files: `internal/git/ops.go` (implied - git integration in handlers)
- Current mitigation: Paths likely constrained to project directory
- Recommendations:
  1. Validate git command paths are within project root
  2. Escape shell arguments to git commands
  3. Test with path traversal attempts (../, etc.)

## Performance Bottlenecks

**Expensive List Rendering with Full Filters:**
- Problem: `handleList` in dataentry applies filters + sorting + pagination to all entities
- Files: `internal/dataentry/handlers.go:49-123`
- Cause: No indexed filtering support; all operations are O(n)
- Current behavior: Filter, sort, then paginate means full iteration even for small result sets
- Improvement path:
  1. Add property value indexing (already exists in graph for faster lookups)
  2. Implement sort-aware iteration (avoid full sort if only need first page)
  3. Consider lazy evaluation for large result sets

**Graph Property Indexing Incomplete:**
- Problem: Graph maintains property index but handlers still iterate all nodes for filtering
- Files: `internal/graph/graph.go:18` (propertyIndex exists), `internal/dataentry/handlers.go:60` (applyFilters on full entity list)
- Cause: Index is built but not exposed through graph query API
- Improvement path:
  1. Add `GetNodesByPropertyValue(property, value)` method to graph
  2. Update handlers to use indexed lookup instead of full iteration
  3. Add benchmark tests to ensure lookups are O(1)

**Large Handler Files Create Hot Paths:**
- Problem: `handlers.go` (2590 lines) and `api_v1.go` (1875 lines) are monolithic
- Files: `internal/dataentry/handlers.go`, `internal/dataentry/api_v1.go`
- Cause: HTTP handlers concentrated in single files
- Impact: Test setup/teardown repeated per test; mental model of handler interactions unclear
- Improvement path:
  1. Split by feature area (entities, relations, analysis, etc.)
  2. Create handler package with sub-packages for each concern
  3. Share test fixtures and utilities

**SSE Event Broker Not Backpressured:**
- Problem: `eventBroker` broadcasts events to all connected clients without checking capacity
- Files: `internal/dataentry/watcher.go:27-29` (clients map)
- Cause: Broadcasts are fire-and-forget; slow clients can block the watcher goroutine
- Impact: Slow browser connections can block file change notifications
- Improvement path:
  1. Add configurable channel buffer size per client
  2. Implement timeout/drop for slow clients
  3. Add metrics to track event broadcast latency

## Fragile Areas

**Workspace RWMutex Over-Locked:**
- Files: `internal/workspace/workspace.go:43` (RWMutex), `internal/workspace/*.go` (write operations)
- Why fragile: Write lock is held for entire UpdateEntity operation including file I/O
- Safe modification:
  1. Lock only the graph mutation, not file operations
  2. Use staging writes with atomic rename
  3. Add deadlock detection tests
- Test coverage: Integration tests cover concurrent updates; unit tests should verify lock bounds

**Event Watcher Subscription Not Leak-Safe:**
- Files: `internal/dataentry/watcher.go` (SSE client management)
- Why fragile: If client channel write fails, client remains registered
- Safe modification:
  1. Add timeout on channel sends
  2. Auto-cleanup after failed sends
  3. Add tests for client disconnection scenarios
- Test coverage: `internal/dataentry/watcher_test.go` has basic tests; missing forced-disconnect scenarios

**Entity Type Discovery by Linear Search:**
- Files: `internal/dataentry/api_v1.go:199-204` (findEntityTypeByPlural)
- Why fragile: Loops through all entities to find type by plural name; O(n) per request
- Safe modification:
  1. Build reverse index: plural → type name at startup
  2. Cache in metamodel loader
  3. Update on metamodel reload
- Test coverage: No specific tests for this lookup

**Live-Reload Race Between File Watch and Manual Request:**
- Files: `internal/dataentry/app.go:55-59` (mu sync.RWMutex)
- Why fragile: Reload acquires Lock while handlers hold RLock; contention under concurrent updates
- Safe modification:
  1. Separate concerns: graph lock vs config lock
  2. Use copy-on-write for config to avoid holding lock during I/O
  3. Add deadlock tests
- Test coverage: `internal/dataentry/watcher_test.go` has concurrent tests; should verify no deadlock under sustained load

## Scaling Limits

**In-Memory Graph Not Suitable for 100k+ Entities:**
- Current capacity: Tested on graphs with 1000-10k entities
- Limit: Linear scan performance degrades significantly beyond 10k entities
- Scaling path:
  1. Short term: Add property-value indexing and cached query results
  2. Medium term: Implement lazy loading or sharding by entity type
  3. Long term: Consider database backend (SQLite, PostgreSQL) if enterprise deployment needed

**AllNodes() Holds RWMutex for Entire Iteration:**
- Current capacity: Acceptable for <5k entities with read workload
- Limit: Long-running queries block all writes
- Scaling path:
  1. Return snapshot of node IDs then fetch individually
  2. Implement stream-based iteration that releases lock between items
  3. Add pagination to graph queries

**SSE Client Map Unbounded:**
- Current capacity: Suitable for <100 concurrent browser connections
- Limit: Memory grows linearly with open connections; no cleanup on disconnect
- Scaling path:
  1. Implement explicit client unsubscribe on close
  2. Add periodic cleanup for dead connections
  3. Add metrics/limits to prevent runaway growth

## Dependencies at Risk

**go-test-coverage Coverage Ratchet Not Updatable by Developers:**
- Risk: `.coverage-baseline` is committed; developers cannot lower coverage without CI intervention
- Files: `.coverage-baseline`, `.testcoverage.yml`
- Impact: Prevents code cleanup if new code paths have lower coverage
- Migration plan:
  1. Document ratchet mechanism clearly (it's in CLAUDE.md)
  2. Add issue type for "coverage improvement" in task tracker
  3. Consider allowing baseline updates with explicit PR approval

**yaml.v3 YAML Parser May Have Security Issues:**
- Risk: No version constraints visible; could be outdated
- Files: `go.mod` (check yaml.v3 version)
- Impact: YAML parsing vulnerabilities could affect metamodel/config loading
- Migration plan:
  1. Run `go list -u` to identify outdated dependencies
  2. Update yaml.v3 to latest version
  3. Test metamodel parsing with fuzzer (already has one)

## Missing Critical Features

**No Automatic Backup or Transaction Support:**
- Problem: Entity mutations are write-through; no rollback on failure
- Blocks: Atomic multi-entity updates impossible; recovery from partial write requires manual intervention
- Fix approach:
  1. Implement atomic writes with staging directory
  2. Add transaction API that batches writes
  3. Document conflict resolution strategy for concurrent updates

**No Version Control Integration for Entity History:**
- Problem: Only current state is stored; no audit trail or history
- Blocks: Cannot answer "who changed this and when?" queries
- Fix approach:
  1. Leverage git integration (already partially implemented)
  2. Parse git log for entity file history
  3. Build UI to browse historical versions

## Test Coverage Gaps

**Dataentry Package at 60% Coverage Threshold:**
- What's not tested: Integration between handlers, live-reload concurrent updates, SSE event flow
- Files: `internal/dataentry/handlers.go`, `internal/dataentry/app.go`, `internal/dataentry/watcher.go`
- Risk: Handler bugs and concurrency issues could pass without detection
- Priority: HIGH - handlers are user-facing and complex
- Recommended tests:
  1. Concurrent create/update/delete operations
  2. Reload under active request load
  3. SSE client lifecycle (connect, receive events, disconnect)
  4. API error responses (invalid input, missing entities, etc.)

**Graph Package at 75% Coverage Threshold:**
- What's not tested: Edge cases in property indexing, trace performance with cycles, concurrent graph mutations
- Files: `internal/graph/graph.go`, `internal/graph/query.go`
- Risk: Graph invariants could be violated in concurrent scenarios
- Priority: HIGH - graph is core data structure
- Recommended tests:
  1. Concurrent AddNode/AddEdge operations with race detector
  2. Property index correctness under mutations
  3. Cycle handling in trace operations (existing tests cover this)
  4. Large graph performance (1000+ nodes, deep paths)

**MCP Server Tests Only Cover Happy Paths:**
- What's not tested: Error handling, malformed inputs, missing resources
- Files: `internal/mcp/` (all handlers marked coverage-ignore)
- Risk: MCP protocol errors could crash AI assistant integration
- Priority: MEDIUM - MCP is integration point
- Recommended tests:
  1. Tool handlers with invalid inputs (missing required fields, wrong types)
  2. Resource handlers for non-existent entities
  3. Prompt generation with missing metamodel data
  4. Connection lifecycle (start, error recovery, shutdown)

**Frontend Components Lack E2E Tests:**
- What's not tested: Vue component interactions, API integration, UI state management
- Files: `frontend/src/` (components, stores, composables)
- Risk: Frontend regressions could break entire UI
- Priority: MEDIUM - UI is user-facing
- Recommended tests:
  1. List view with filters and pagination
  2. Entity create/edit/delete workflows
  3. Graph visualization rendering with large data
  4. Keyboard shortcuts and keyboard navigation

---

*Concerns audit: 2026-03-19*
