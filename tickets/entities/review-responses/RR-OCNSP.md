---
id: RR-OCNSP
type: review-response
title: writeDataFile calls s.fs.MkdirAll — tied to RR-HK9G8
finding: markdown.go:518 calls s.fs.MkdirAll using raw FS. Correct today (MkdirAll is directory topology). StoreFS has no MkdirAll; DirFS has it. If s.fs were typed DirFS (per RR-HK9G8), this line would compile; if StoreFS, it would not — exactly the compile-time check the design wanted.
severity: nit
resolution: s.dirs is now typed DirFS (has MkdirAll). MkdirAll is an intentional part of DirFS — it's directory-topology. Bundled with the parent fix RR-HK9G8.
status: addressed
---
