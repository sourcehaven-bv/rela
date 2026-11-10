---
id: RR-NM593GM
type: review-response
title: Comment overclaims that no DSN reaches the log
finding: An open error can carry host, user and database.
severity: nit
resolution: Comment now says the error is logged only and may name host, user and database; pgx redacts the password.
status: addressed
---
