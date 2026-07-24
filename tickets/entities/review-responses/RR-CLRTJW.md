---
id: RR-CLRTJW
type: review-response
title: --out <dir> wastes a full browser build before erroring; message names wrong flag
finding: 'cranky (2nd pass): the --out-is-a-directory check ran AFTER docs.Build, so a screenshot{} manual would spin up headless Chrome and drop stray PNGs in the cwd only to fail at the final write. Separately, the error text said ''--output'' but the kong flag is ''out''.'
severity: minor
resolution: 'Hoisted the os.Stat/IsDir check to the top of Run (right after reading the manual, before any build work) and corrected the message to ''--out %q is a directory''. Strengthened TestBuild_OutputIsDir: a screenshot manual with a capturer factory that fails the test if invoked proves Run rejects before the build, and asserts no stray PNG is written.'
status: addressed
---
