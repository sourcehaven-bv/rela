---
id: RR-3TQABD
type: review-response
title: Path guard does not protect the project directory
finding: sandbox_linux.go:28 still promises the project dir, .rela secrets, /root and /home are not present, but that now holds only if the operator lists nothing covering them. Debian needs /var/lib/texmf and /var/lib/fontconfig; an operator who lists /var/lib passes unsafeReadPath and exposes a project under /var/lib/rela, .rela secrets and Postgres data to \\input{} disclosure. Check at the composition roots (dataentry.NewApp, rela render) that no host path is, contains or lies inside the project root; at minimum fix the comment.
severity: significant
resolution: 'New cmdexec.CheckProjectNotExposed(root): error when an operator read path is, contains, or lies inside the project directory, symlinks resolved on both sides. dataentry.NewApp returns it as an error (rela-server and rela-desktop refuse to load the project) and rela render returns it before running a converter. unsafeReadPath additionally refuses any path at or above /var/lib, /home, /root, /etc and /run, so listing /var/lib to cover texmf and fontconfig is refused with a pointer to list single entries. sandbox_linux.go comment now states the guarantee depends on these checks. TestCheckProjectNotExposed covers project, parent, inside, symlink and sibling.'
status: addressed
---
