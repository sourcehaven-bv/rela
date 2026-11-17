---
id: RR-IEZT6ZA
type: review-response
title: Field-by-field query copies can drop or overwrite Where
finding: narrowed, nav status, appbuild lowering, visibility page/rollup/header queries and backend splits copy GraphQuery fields. AND Where through a helper, reject Where in rollup gates, and cover counts, MatchingFaces and headers.
severity: significant
resolution: store.AndWhere combines trees without aliasing; listpushdown narrowed and visibility readPage use it; rollup gates reject Where; pageViaFilter refuses Where. The store differential compares counts, MatchingFaces and headers with Where set.
status: addressed
---
