---
id: RR-PAF29J
type: review-response
title: No migration of existing .rela/comments/ YAML into a database
finding: 'An operator who commented before switching to the postgres or sqlite build finds their comments gone: the YAML files are intact on disk, invisible to the app, with no warning. Shipping a second flavour of "your comments are invisible" is a pointed thing to do in the ticket that exists to fix the first one.'
severity: minor
reason: 'Explicitly out of scope on TKT-OGTVJW: "No migration of existing .rela/comments/ YAML into a database — an operator adopting postgres starts with an empty comment store, as they do for every other table." That is the same deal every other table makes (entities, relations, attachments and search all start empty on a backend switch), so comments are not being singled out. A one-shot import belongs in internal/datamigration alongside the other adoption paths, which is a ticket of its own rather than a widening of this one. The user-facing guide now states plainly where each backend keeps comments, so the behaviour is at least discoverable before an operator switches.'
status: deferred
---
