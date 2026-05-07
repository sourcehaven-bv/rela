---
id: RR-KX7ER
type: review-response
title: Real anchor will inherit default link styling unless overridden
finding: 'Existing .list-link rules style cursor + hover color on .entity-title only. With a real <a href>, default browser styles (blue underline, visited purple) apply unless overridden. Add text-decoration: none; color: inherit; to .list-link.'
severity: nit
resolution: 'Plan adds text-decoration: none; color: inherit; to .list-link CSS to suppress default browser link styling.'
status: addressed
---
