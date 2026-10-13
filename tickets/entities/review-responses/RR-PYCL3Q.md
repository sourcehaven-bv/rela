---
id: RR-PYCL3Q
type: review-response
title: Endpoints missing from the corpus
finding: _position, _sidepanel, _relation_history, _copies, _piles, _comments, _documents, _commands, _analyze, _next_action, _action/, _feeds and hidden-field filter/sort, ?world=, bare-id ambiguity are absent.
severity: significant
resolution: Plan lists them and adds a guard that fails for any router-walk route without a case or exemption.
status: addressed
---
