---
id: RR-8ULC8I2
type: review-response
title: Lua write_file writes to root/output on disk in an in-memory world
finding: 'Lua rela.write_file (internal/lua/output.go) calls os.WriteFile under ProjectRoot/output; an automation with write_file: true would create files under the project root from a NewInMemory world.'
severity: critical
reason: 'TKT-NRNDGZR (part 2) owns the output dir: WithOutputDir and write_file honouring it are in its acceptance criteria. NewInMemory has no non-test caller until part 4 wires rela test. Building the seam here would duplicate part 2.'
status: deferred
---
