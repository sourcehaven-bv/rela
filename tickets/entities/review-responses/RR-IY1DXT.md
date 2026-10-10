---
id: RR-IY1DXT
type: review-response
title: configedit import graph and validation parity
finding: dataentry may not import appbuild; preview validation differing from NewApp's could pass a draft the rebuild rejects.
severity: significant
resolution: 'Plan changed: configedit defines narrow consumer interfaces (Validator, Rebuilder) supplied by main; preview validates with the same constructors the rebuild uses (SharedBase over an overlay FS plus the App''s config load), before anything is written.'
status: addressed
---
