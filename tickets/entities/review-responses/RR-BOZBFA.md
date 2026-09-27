---
id: RR-BOZBFA
type: review-response
title: JSON path spelling and escaping
finding: ->> vs json_extract and '$.p' vs '$."p"' do not match each other; & in keys; schema names may contain a double quote.
severity: minor
resolution: One jsonPath helper everywhere (verified by probe that json_extract does not match a ->> index); tests with & and ' in names; names with a double quote fall back to naive.
status: addressed
---
