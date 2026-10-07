---
id: RR-GOL8UR
type: review-response
title: '[security] Refusal list omits the hazards the docs say never to list'
finding: 'Docs say never list /run or /var/run, but the code accepts them, plus /var, /var/lib, /home and /root. /var/lib is one segment from the documented /var/lib/texmf and the parent of a project at /var/lib/rela/project, so a shortened entry lets any markdown author \\input .rela/secrets.yaml into a PDF. /run exposes the Postgres socket with peer auth as rela''s uid. Fix: refuse /run, /var/run and ancestors of /var/lib; refuse or warn on paths containing or inside the project dir.'
severity: minor
resolution: unsafeReadPath now refuses any path at or above /run, /var/run, /var/lib, /home and /root (in addition to /, /etc, /proc, /dev, /tmp and the temp dir), and CheckProjectNotExposed refuses paths that are, contain or lie inside the project directory at every entry point that has a project (dataentry.NewApp, rela render). transforms.md lists exactly what is refused.
status: addressed
---
