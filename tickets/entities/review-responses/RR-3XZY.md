---
id: RR-3XZY
type: review-response
title: Long-bracket comments at level >0 bypass leading-return preprocessor
finding: 'preprocess.go startsWithReturnKeyword only recognizes --[[ ... ]] block comments. Lua''s lexer accepts --[=*[ ... ]=*] for any level. Source like `--[==[ banner ]==]return false` slips past the leading-return check, gets `return ` prepended, and surfaces as a confusing ''syntax error near return'' instead of the clear leading-return rejection that''s the preprocessor''s purpose. Fix: parse the leading [=*[ count and find the matching ]=*]. Add testdata/reject/leading_return_after_long_bracket.lua.'
severity: significant
resolution: Rewrote preprocess.startsWithReturnKeyword to handle long-bracket comments at any level via matchLongBracketOpen / findLongBracketClose helpers. Count `=` characters between the two `[`s, match the same number in the closing `]=*]`. Added testdata/reject/leading_return_after_long_bracket.lua covering the `--[==[ banner ]==]return false` form. AC2 corpus runner catches it.
status: addressed
---
