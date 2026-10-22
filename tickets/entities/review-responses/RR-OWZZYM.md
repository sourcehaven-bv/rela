---
id: RR-OWZZYM
type: review-response
title: mappingKeys must handle aliases and merge keys
finding: 'Parse decodes session.states into a Go map, and a separate walk of yaml.Node gives the order. With an alias (states: *base) or a merge key (<<: *common) under session.states, the node walk and the decoded map disagree (it sees "<<", or an AliasNode). Follow aliases and expand merge keys in the helper, or refuse both under session.states. Add a test row.'
severity: nit
resolution: 'Plan updated: mappingKeys follows aliases and refuses merge keys under session.states with a clear message; one test row each.'
status: addressed
---
