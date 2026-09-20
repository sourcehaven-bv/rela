---
id: RR-GX7PIJ
type: review-response
title: A directory minted a download token and returned 200 with an empty body
finding: 'containedProjectPath validates WHERE a path is, not WHAT it is. A script emitting a `file` message naming a directory passed containment, minted a token, and on download os.Open succeeded — so setHardenedDownloadHeaders was written and only then did io.Copy fail with ''is a directory''. Verified by probe: code=200, bodylen=0. The user gets a zero-byte file named after the directory and no error surfaces anywhere except a server log line, because the status was already committed.'
severity: significant
resolution: Added an os.Stat + Mode().IsRegular() check in mintFileToken, immediately after containment succeeds. Checked at mint rather than at download so the operator gets the same warning as any other unusable path and the download path stays a straight read. Pinned by TestMintFileToken_RejectsDirectory.
status: addressed
---
