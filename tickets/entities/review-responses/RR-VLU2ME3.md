---
id: RR-VLU2ME3
type: review-response
title: Seen ids remain in localStorage after sign-out
finding: The seen set holds entity type and anchor ids; another person on the same browser profile could read which entities the user could see.
severity: minor
reason: 'The set is now cleared on a refusal. Sign-out is a full page load the SPA cannot hook, as for recentEntities.ts, which stores ids the same way. Hashing does not hide existence: ids are guessable, so anyone with a candidate id can test it against the hash. Reading another profile''s storage needs local access to that profile.'
status: wont-fix
---
