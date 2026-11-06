---
id: RR-JV1L84I
type: review-response
title: Re-marshalling escapes HTML and grows states
finding: json.Marshal escapes <>& to six bytes each, so a state near 1 MiB could exceed the limit after stamping.
severity: nit
resolution: The encoder uses SetEscapeHTML(false); a table case checks <&> round-trips.
status: addressed
---
