---
id: RR-EXIC51
type: review-response
title: content on create_relation was documented, wired, and unreadable
finding: The opts table's `content` key reached the store correctly (entitymanager/manager.go:1913 honours opts.Content) but relationToTable emitted from/type/to/from_face/properties and NOT content, so `rela.create_relation(a, t, b, {content = "x"}).content` was nil. The key was newly documented in the guide's options table as supported, and a script author's first move is to read it back. It also had zero test coverage — every test covered `face`. Pre-existing that relationToTable omitted content (verified against HEAD), but this change made it worse by advertising the key as writable, turning a quiet omission into a write-only trap.
severity: significant
resolution: 'Added `content` to relationToTable, set unconditionally like from_face, with a comment stating that a key a script can set but never read back is a trap. Added TestCreateRelation_ContentRoundTrips covering the write-then-read path. Kept the key rather than dropping it: it retires the phantom positional `content?` argument the old doc comment advertised and the body never read, so removing it would leave that wart in place.'
status: addressed
---
