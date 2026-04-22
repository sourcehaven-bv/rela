---
id: RR-3Z7EI
type: review-response
title: tsconfig excludes pages/ from type-checking
finding: 'rootDir: ./tests and include: tests/**/*.ts mean `tsc --noEmit` errors with TS6059 on every pages/*.ts. Page-object layer is a type-free zone. C1 and C2 escaped because of this.'
severity: critical
resolution: 'tsconfig.json rewritten: dropped rootDir; include now covers tests/**/*.ts + pages/**/*.ts. Added typescript dep + `typecheck` npm script. CI e2e job runs typecheck after lint.'
status: addressed
---
