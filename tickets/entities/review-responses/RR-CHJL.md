---
id: RR-CHJL
type: review-response
title: EnvPrincipalResolver re-reads $RELA_DATAENTRY_USER per request
finding: Per-request os.Getenv is sync.RWMutex.RLock + map lookup — cheap, but pointless if env doesn't change at runtime. Either cache at construction or document the choice.
severity: minor
resolution: 'Added a one-line doc comment on EnvPrincipalResolver explaining the per-request read is intentional — it lets t.Setenv in tests take effect without resolver reconstruction. Cost: one RLock per request. File: internal/dataentry/router.go:158-170.'
status: addressed
---
