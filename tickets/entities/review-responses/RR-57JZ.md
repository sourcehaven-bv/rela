---
id: RR-57JZ
type: review-response
title: 'F14: root.go error wrap is scope creep and can produce misleading messages'
finding: 'TKT-YBKB included a drive-by edit to internal/cli/root.go that wrapped the workspace.Discover error unconditionally with ''no project found: %w (run rela init to create one)''. The hint was attached even when the underlying error was a permission denied or a corrupt-metamodel parse failure, which is actively misleading.'
severity: nit
resolution: 'Resolved by the rebase onto develop: PR #326 (fix(cli): surface real project-discovery errors) landed a proper wrapDiscoverError that uses errors.Is(err, errors.ErrNoProject) to attach the hint only when the project is genuinely absent. My drive-by edit was dropped during conflict resolution; develop''s version is now in place.'
status: addressed
---
