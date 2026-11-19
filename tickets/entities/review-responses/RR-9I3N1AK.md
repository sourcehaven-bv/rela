---
id: RR-9I3N1AK
type: review-response
title: JSON null decodes to Off
finding: ChromeToggle.UnmarshalJSON turned a JSON null into Off instead of the default.
severity: minor
resolution: A null leaves the toggle at its default. TestChromeToggle_JSONNullIsDefault.
status: addressed
---
