---
id: RR-Z6NG6HD
type: review-response
title: Picker reads treat every error as not found
finding: RelationDefaultPicker swallowed every getEntity error and moved to the next type, so a 500 or network drop showed 'not found'; each saved default probed every target type in turn.
severity: significant
resolution: 'Only a 404 moves on to the next type; any other error shows ''could not load''. Types whose id prefix the id carries are tried first, so a saved default normally costs one read. A server-side batch of saved titles was not added: the ticket asks for a client read and the number of saved defaults is small and operator-authored.'
status: addressed
---
