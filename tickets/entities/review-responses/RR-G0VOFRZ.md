---
id: RR-G0VOFRZ
type: review-response
title: Nil stamp result silently removes the client
finding: A stamp returning nil encoded as null, turning a live update into a removal.
severity: nit
resolution: stampState returns errStampNil and the frame is dropped; godoc says the stamp must not return nil; TestStampAwareness_NilStampIsRefused.
status: addressed
---
