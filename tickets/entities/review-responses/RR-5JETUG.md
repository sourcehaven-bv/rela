---
id: RR-5JETUG
type: review-response
title: One undecodable row blocks opening the database
finding: reindexSearchText runs in sqlitestore.New. A row whose properties fail unmarshalProps makes New fail with an error that does not name the row, which also blocks rela db dump.
severity: minor
resolution: 'The error now names the entity id and face. Opening still fails: skipping the row would leave it silently out of the index, which is the class of silent failure this bug fixed.'
status: addressed
---
