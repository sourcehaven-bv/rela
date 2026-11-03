---
id: RR-2OB32D
type: review-response
title: Item text bypasses the field write gate
finding: Mapped values are treated as operator-written, but {{item.text}} is caller input; a principal who may not write title could write it via a recipe.
severity: significant
resolution: 'Plan: checkCreateFieldWrite runs on every target field whose template references item.text plus caller properties; test with a write-denied mapped field.'
status: addressed
---
