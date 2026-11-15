---
id: RR-ZSJYL38
type: review-response
title: Seed via w.create instead of the store
finding: The test seeds through w.st.CreateEntity; reviewer suggested w.create with a fixed id.
severity: minor
reason: 'w.create cannot set the id: control has id_type short and the manager rejects custom ids. The sibling RealPolicy test seeds through the store for the same reason.'
status: wont-fix
---
