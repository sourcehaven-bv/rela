---
id: RR-9AW9DH
type: review-response
title: Directory guard is a filename-extension heuristic, not a containment check
finding: 'TestDefaultScannerConfigsAreAbsoluteFiles rejects a directory via filepath.Ext(p) == "", which is a naming convention rather than a property of the path: /etc/clamav/conf.d passes the guard and IS a directory, while a legitimately extensionless config file would be wrongly rejected. The comment claims the test guards that "none may be a bare directory", which is stronger than what the code checks. Minor rather than significant because it is defense-in-depth over a hardcoded constant list edited by humans, not runtime input — and it did empirically catch a real transient mid-edit state where /etc/clamav appeared in the list, with the intended message. Fix: keep the extension check as a cheap smell test and add the honest one — when the path exists on the test host, os.Stat it and fail on IsDir(), so the test asserts the property its comment claims on any machine that actually has ClamAV while staying green elsewhere.'
severity: minor
status: addressed
resolution: 'TestDefaultScannerConfigsAreAbsoluteFiles now does the honest check in addition to the heuristic: for each path it os.Stat()s and fails on IsDir(), skipping (continue) when the path does not exist so the test stays green on hosts without ClamAV. The extension check is retained as a cheap host-independent smell test, with a comment stating explicitly that it is not sufficient on its own and naming /etc/clamav/conf.d as the case it would miss. The failure message now names the actual consequence (exposing signature databases and freshclam.conf, which can carry a DatabaseMirror credential).'
---
