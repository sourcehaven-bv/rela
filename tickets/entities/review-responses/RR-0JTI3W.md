---
id: RR-0JTI3W
type: review-response
title: dataentry projectInfo bypasses the injected FS and stats disk per request
finding: 'commands.go projectInfo() constructs a fresh storage.NewSafeFS(storage.NewOsFS()) on every call and stats the disk on a request path. dataentry otherwise uses an injected storage.FS, so this reaches around it to the real OS filesystem — under a MemFS-backed test app it silently reads a different filesystem than the rest of the request. Unnecessary regardless: Context.SchemaPath already holds the resolved answer from discovery, so filepath.Base(paths.SchemaPath) gives the same result with no syscall and no FS-injection escape hatch. Consequence of my having reached for a helper instead of the value that was already computed.'
severity: significant
resolution: Removed the per-request stat and the FS bypass entirely. Added App.SchemaFileName() returning filepath.Base(a.paths.SchemaPath) — the value already resolved at discovery — and wired it into commandHandler as a schemaFile closure alongside the existing projectRoot one, matching the package's established closure-over-App pattern. projectInfo() now reads that closure (falling back to the canonical name when unset) instead of constructing storage.NewSafeFS(storage.NewOsFS()). No syscall on the request path, no reaching around the injected FS, and the two dataentry imports added earlier are gone.
status: addressed
---
