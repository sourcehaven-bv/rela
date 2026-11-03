---
id: RR-AIKRO1
type: review-response
title: Unsupported browser languages lose their date and number format
finding: When no language matched, formatting became en (US style), while before the change a de-DE browser formatted in de-DE.
severity: significant
resolution: On the English fallback the formatting locale is now the first browser tag, as before. A chosen language still formats in that language; documented in language.ts. Test row de-DE added.
status: addressed
---
