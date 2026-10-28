---
id: RR-ODAIO1
type: review-response
title: widthClass could be a lookup table
finding: A Partial<Record> lookup would read more simply than the function.
severity: nit
reason: A record lookup on a server string also resolves inherited keys such as constructor; the two explicit comparisons avoid that and are tested for an unknown value.
status: wont-fix
---
