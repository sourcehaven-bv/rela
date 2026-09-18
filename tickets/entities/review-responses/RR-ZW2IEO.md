---
id: RR-ZW2IEO
type: review-response
title: Churn-freedom was tested on the path where it is true by construction
finding: 'guardWriteBack returns the original bytes whenever dirty is false, so the ''round-trips a table
  and a list'' test passed without consulting the serializer at all and would pass with one that emitted
  garbage. The real behaviour was untested and undocumented: once the user edits anything the guard steps
  aside and the app receives a full reserialization, including reformatting of blocks nobody touched.
  That is a behaviour change from EasyMDE, which was a plain text buffer that returned exactly what was
  typed.'
severity: significant
resolution: Split into two tests. One pins that an UNEDITED body comes back byte-identical, using only
  constructs the round-trip would rewrite so it fails if the guard is bypassed. The other pins that one
  command on the last block repads an untouched table, asserting the change rather than leaving it to
  be discovered. Documented on the value getter and in the custom-apps guide.
status: addressed
---
