---
id: RR-2ULPMU
type: review-response
title: 'Security: one read-path list exposes the clamd socket to converters'
finding: 'RELA_SANDBOX_READ_PATHS is one list bound into every sandboxed command. A scanner needs the clamd socket and clamd.conf in it, so export converters and attachment transform steps that parse untrusted content can connect to clamd too: SHUTDOWN stops scanning (every scanned upload is then rejected) and SCAN <path> probes files clamd can read. Before this ticket only the scanner had the socket (attachments.scan_sockets). The change weakened isolation; the docs only warned about it.'
severity: significant
resolution: 'Read paths are per cmdexec.Purpose: RELA_SANDBOX_TRANSFORM_READ_PATHS (export transforms, attachment transform steps, document commands) and RELA_SANDBOX_SCAN_READ_PATHS (attachment scans), with matching rela-server flags. attachment.CmdRunner holds one runner per purpose. CheckProjectNotExposed and the empty-list warning cover both lists. TestHostReadOnlyIsPerPurpose and TestCmdRunnerBindsOnlyOperatorPaths pin the isolation.'
status: addressed
---
