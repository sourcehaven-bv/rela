---
id: RR-ATHC2
type: review-response
title: Numeric-string coercion accepts hex/binary/Infinity/whitespace, diverging from decimal 'numeric string' contract and from predicate
finding: 'toNumber delegates to JS Number(), which parses more than decimal numerals: ''0x10''==16, ''0b101''==5, ''  5  ''==5, ''Infinity'' coerces to a finite-comparable infinity. The stated contract is ''number matches numeric string''; these edge cases are almost certainly not intended and will diverge from the Go internal/predicate path the grammar claims congruence with. Gate toNumber on a decimal regex (^-?\d+(\.\d+)?([eE][+-]?\d+)?$ after trim) for strict decimal parity, or document the permissiveness. Currently untested.'
severity: significant
resolution: toNumber now gates string coercion on DECIMAL_RE (^[+-]?\d+(\.\d+)?([eE][+-]?\d+)?$) instead of raw Number(), and rejects non-finite. Hex ('0x10'), binary ('0b101'), whitespace ('  5  '), and 'Infinity' no longer coerce to a decimal literal, restoring parity with the decimal-only 'numeric string' contract and internal/predicate. Tests assert decimal/fraction/exponent/sign accepted and hex/binary/whitespace/Infinity rejected.
status: addressed
---
