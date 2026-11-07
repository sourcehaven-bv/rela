---
id: RR-X3V6OMC
type: review-response
title: Services.FS doc still says project filesystem
finding: For NewInMemory Services.FS is an empty MemFS; readers of operator files through it would see no config.
severity: significant
resolution: Services.FS doc now says it is the runtime-state FS; the NewInMemory godoc states the split and points config readers to the config loader.
status: addressed
---
