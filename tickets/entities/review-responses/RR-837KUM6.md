---
id: RR-837KUM6
type: review-response
title: '[security] A fixture path could leave the project through a symlink'
finding: fsimport opens the fixture directory with os.OpenRoot, which follows a symlink in its name.
severity: minor
resolution: loadFixture calls noSymlinks on every component before fsimport.Copy. TestFixtureSymlinkRefused covers it.
status: addressed
---
