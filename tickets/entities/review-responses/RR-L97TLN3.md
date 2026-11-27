---
id: RR-L97TLN3
type: review-response
title: BuildFieldWriteGate has no direct test
finding: The exported builder is reached only through the golden corpus.
severity: minor
resolution: 'Added TestBuildFieldWriteGate in internal/appbuild/fieldgate_test.go: nil, NopACL and ReadOnlyACL give AllowAllFieldGate; a Declarative with field grants refuses status and allows title.'
status: addressed
---
