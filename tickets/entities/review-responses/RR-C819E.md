---
id: RR-C819E
type: review-response
title: Large TTL values silently overflow to immediately-expired
finding: 'internal/lua/cache.go: opts.ttl = time.Duration(f * float64(time.Second)) where f is user-supplied ttl. For f > math.MaxInt64/1e9, the float64->int64 conversion wraps to a huge negative duration; the entry is born already-expired and the next get returns a miss without any error. Script author has no diagnostic signal.'
severity: significant
resolution: 'Added maxCacheTTLSeconds constant (~9.2e18 via math.MaxInt64/time.Second) and reject ttl > that value at parse time with ''cache: ttl too large (max <n> seconds)''. New test: TestCacheRejectsOversizedTTL covers ttl=1e20.'
status: addressed
---
