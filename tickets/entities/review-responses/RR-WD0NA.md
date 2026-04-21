---
id: RR-WD0NA
type: review-response
title: relativize swallowed filepath.Rel errors and leaked absolute paths
finding: The Walk/WalkAll callback remapper had a defensive fallback that passed the original absolute path through on filepath.Rel failure, silently violating the 'never absolute paths' contract. Untested branch.
severity: significant
resolution: 'Removed the fallback. relativize now returns fmt.Errorf(''rooted: callback path %q not under root %q: %w'', path, root, err) when filepath.Rel fails. No silent contract violation; error propagates through Walk.'
status: addressed
---
