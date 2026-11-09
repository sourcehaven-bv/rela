---
id: RR-VC0JS6R
type: review-response
title: rela validate repeats subscription errors on the peers line
finding: validatePeers used LoadConfig and errors.Is on a joined error.
severity: nit
resolution: validatePeers calls LoadPeers and AddPeers on an empty Config. TestValidateWithFS_PeersStandAlone.
status: addressed
---
