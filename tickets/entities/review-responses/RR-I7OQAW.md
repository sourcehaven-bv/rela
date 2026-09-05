---
id: RR-I7OQAW
type: review-response
title: List's not-a-directory behaviour diverged between OsFS and MemFS
finding: 'FSLoader.List forgave any error matching os.ErrNotExist. What that covers depends on the injected storage.FS: OsFS returns ENOTDIR for a path that exists but is a regular file (surfaces, correct), while MemFS returns os.ErrNotExist for anything absent from its directory map (forgiven, wrong). So ''scripts exists but is a file'' silently reported ''no scripts'' on MemFS — precisely the silently-drop-operator-config failure the absent-is-empty asymmetry exists to prevent, made dependent on which FS happened to be installed.'
severity: significant
resolution: The absent check is now an explicit fs.Stat before ReadDir, distinguishing absent (empty, nil error) from exists-but-not-a-directory (error) without relying on either backend's error mapping. Interface doc updated to state the rule. Added TestFSLoader_List_NotADirectoryIsAnError.
status: addressed
---
