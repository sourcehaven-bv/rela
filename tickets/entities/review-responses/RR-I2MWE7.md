---
id: RR-I2MWE7
type: review-response
title: Axis key normalises 03 to 3
finding: The cell link filters on 3 while a string 03 is stored.
severity: nit
reason: A zero-padded integer string only comes from a hand edit. Counting it with 3 is what a reader expects; the cell link then omits that one row, which is accepted as rare.
status: wont-fix
---
