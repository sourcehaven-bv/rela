---
id: RR-N61P1I
type: review-response
title: Broken channel store turns a 403 into a 500
finding: openScope reads the channel store before the authorization pre-pass, so a store error gives 500 to a caller who would get 403.
severity: nit
reason: A broken channel store gives 500 instead of 403 to a caller who would be denied; nothing leaks. Reading the channel store first lets deletes without channels skip the duplicated authorization pre-pass, which every delete would otherwise pay.
status: wont-fix
---
