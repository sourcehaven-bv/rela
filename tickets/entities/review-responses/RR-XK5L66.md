---
id: RR-XK5L66
type: review-response
title: MaxPerTarget is a check-then-act race once several processes share a database
finding: Service.Add enforces MaxPerTarget by listing the thread, counting, then inserting. That is non-atomic. On filecomments a process-local mutex made it nearly safe; with several rela-server processes against one database — the topology pgcomments exists to serve — there is no mutex anywhere, so N concurrent posts to a thread at the limit all read the same count and all insert. The ticket's premise is that this tier is now genuinely multi-writer, and this is the one invariant in the comment service that assumed it was not.
severity: minor
reason: Documented rather than fixed, deliberately. The cap exists to bound the FILE backend, which holds a whole thread in one document read in full on every List; overshooting it by a handful of rows on a backend that pages costs nothing real. Making it exact would mean a conditional INSERT plus a RowsAffected check, which needs a new method on comments.Store — a contract change, and a widening of the interface this ticket's scope explicitly froze ("no interface change", AC1), for an invariant that does not need to be exact. The weakening is now recorded on the MaxPerTarget declaration itself, where someone relying on it will read it, rather than left for the next person to discover.
status: wont-fix
---
