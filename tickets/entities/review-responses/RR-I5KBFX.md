---
id: RR-I5KBFX
type: review-response
title: Tokens whose release never landed were permanently unreclaimable
finding: 'The expiry model gave entries a zero deadline at mint, and only release() set one. expired() returned false for a zero deadline, so any entry whose release never ran could never be swept — verified by probe: 1001 entries still resident after 10,000x the TTL. Latent rather than live, since release is deferred before the first mint on every path today, but it meant the table had exactly one reclamation path and it was opt-in. The doc comment sold the zero-deadline design as deliberate without stating its cost.'
severity: minor
resolution: Every entry now gets an absolute deadline (commandFileMaxLifetime, 12h) at mint; release shortens it to now+TTL rather than setting it from nothing. expired() drops to a one-liner. The in-flight-runs-outlive-the-short-TTL property is preserved for any run shorter than the ceiling. release also refuses to move a deadline later, so finishing cannot grant a fresh window. Pinned by TestCommandFileStore_UnreleasedTokensStillExpire and _ReleaseOnlyShortens.
status: addressed
---
