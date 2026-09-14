---
id: RR-1TQPWE
type: review-response
title: lock.ValidateKey doc claims parity with state.ValidateKey but is strictly weaker
finding: |-
    internal/lock/lock.go's ValidateKey doc said the rules "mirror state.ValidateKey rather than inventing a second dialect, minus the filesystem-specific clauses", and concluded: "a caller deriving a lock key and a state key from the same source should not have to remember two contracts." That conclusion is false, and it is the part a caller would rely on.

    Verified by running both validators over one key table:

      "has:colon"      lock=nil  state=colon not allowed
      "con"            lock=nil  state=Windows reserved name
      "nul.txt"        lock=nil  state=Windows reserved name
      "a/con/b"        lock=nil  state=Windows reserved name
      "COM1"           lock=nil  state=Windows reserved name
      "lpt9.log"       lock=nil  state=Windows reserved name

    lock.ValidateKey is STRICTLY WEAKER: it omits state's colon rule, the Windows-reserved-name rule, and the explicit leading-slash rule (a leading slash is caught only incidentally, as an empty segment). It adds a length bound state does not have.

    The "minus the filesystem-specific clauses" hedge does not cover this honestly. Colon and Windows reserved names ARE the filesystem clauses, so the sentence is self-consistent, but the following sentence then tells the reader the two contracts are interchangeable — which is exactly the inference that breaks. A caller who validated a webhook-derived key with lock.ValidateKey and then used it as a state key would get a runtime rejection the doc promised could not happen.

    Not a correctness bug in the lock itself: no Locker backend resolves a key to a path today, so the weaker rule is adequate for both current backends. The defect is the documented contract, which is the thing callers program against.

    The same false claim appeared in locktest.go's invalid-key table comment ("Held to the same table as state.ValidateKey so a caller ... meets one contract, not two"), which would mislead the author of the next backend into thinking the suite enforces parity it does not test.
severity: minor
resolution: |-
    Documentation corrected rather than the rules changed — tightening lock.ValidateKey to full state parity would reject keys (colons especially) that are perfectly safe for a hash-input or map-key backend, for the benefit of a filesystem backend that does not exist.

    lock.go's doc now states the relationship precisely: the rules are state.ValidateKey's minus the path-only clauses plus a length bound, i.e. STRICTLY WEAKER, naming the three concrete divergences and the example keys. It explicitly warns that a key valid here is not necessarily valid as a state key and that a caller deriving both must validate against state.ValidateKey, the stricter of the two. It also records that a future filesystem-backed Locker must bring the missing clauses with it.

    locktest.go's table comment now says it is a SUBSET of state's table — the cases every backend must reject — rather than claiming one shared contract.
status: addressed
---
