---
id: RR-YC94B00
type: review-response
title: "Branding is read on every render"
finding: "mailbrand.Source reads data-entry.yaml and the Settings palette on each mail.render call, so a script that renders many mails reads them many times."
severity: nit
reason: "Both reads are small and local; caching would need invalidation on config reload and palette saves for a cost no caller has shown."
status: wont-fix
---
