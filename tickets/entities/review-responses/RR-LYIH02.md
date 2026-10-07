---
id: RR-LYIH02
type: review-response
title: git-crypt warning covers only the database
finding: Project files matched by a git-crypt pattern are copied decrypted and would be committed in clear by a new repo; only the top-level .gitattributes was checked.
severity: minor
resolution: Every copied .gitattributes is checked, and the warning now names project files and the need to set up git-crypt before committing.
status: addressed
---
