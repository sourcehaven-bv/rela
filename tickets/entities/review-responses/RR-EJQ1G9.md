---
id: RR-EJQ1G9
type: review-response
title: Service.Messages and MessagePage are now production code kept only for tests
finding: service.go:207 delegates to Read, and no non-test caller remains (record_test, refs_test, ops_test, service_test). The plan said to remove it or delegate; it is dead exported API. Move those tests to Read and delete it.
severity: minor
resolution: Deleted both. Their tests now call Service.Read (TestMessages_PageClamp became TestRead_PageClamp).
status: addressed
---
