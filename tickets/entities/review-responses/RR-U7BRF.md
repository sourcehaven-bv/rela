---
id: RR-U7BRF
type: review-response
title: 'F8: denylist redaction misses common secret shapes'
finding: 'Plan''s regex (?i)password|token|secret|api[_-]?key misses: authorization, bearer, cookie, session, credential, pat, client_secret, private_key, webhook_url. Also no value-shape redaction for JWT-like (eyJ...) or long random hex/base64. Easy to broaden now; no reason to defer.'
severity: significant
resolution: 'Denylist broadened: password, token, secret, api[_-]?key, authorization, bearer, cookie, session, credential, pat, client[_-]?secret, private[_-]?key, webhook[_-]?url. Added value-shape regexes for JWT (^eyJ[A-Za-z0-9_=-]+\.), long hex (32+ chars), long base64 (32+ chars). Tests cover each.'
status: addressed
---
