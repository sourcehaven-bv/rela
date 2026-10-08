---
id: RR-U42OLN
type: review-response
title: 'Design: a listed directory exposes its unix sockets for connect, not only reads'
finding: '[security] A read-only bind does not prevent connect() on a unix socket (Linux), and on macOS each entry becomes an allow network-outbound unix-socket subpath rule. Listing /run, /var/run or /opt/homebrew/var to reach clamd hands every converter fed untrusted content the Postgres socket (peer auth as the rela user), docker.sock and the system D-Bus. Docs, flag help and godoc describe the exposure only as readable; transforms.md says the variable is Linux-only although on macOS it controls socket connects.'
severity: significant
resolution: 'SetHostReadOnly godoc, --sandbox-read-paths help, docs/transforms.md and the attachment-security guide now say listed paths are readable AND their unix sockets connectable, and to list the socket file, never /run or /var/run. transforms.md no longer implies the variable is Linux-only: on macOS it selects the connectable sockets. No startup socket scan: walking an operator directory at boot is cost for little gain once refusing / and documenting the file-not-directory rule.'
status: addressed
---
