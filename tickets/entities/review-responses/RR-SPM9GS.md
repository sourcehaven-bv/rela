---
id: RR-SPM9GS
type: review-response
title: TestMIME_ZipContainerExtensionsResolve asserts a host property, so it passes where it matters least
finding: The test asserts mime.TypeByExtension maps each container extension into zipContainerMIMETypes. CI runners (ubuntu-latest) ship /etc/mime.types via mailcap and macOS has /etc/apache2/mime.types, so it passes in both places; the production container is the one host where it would fail, and there it never runs. Correct instinct, wrong layer — it is what lets RR-7C5D7T reach production green.
severity: minor
resolution: Obviated, as the reviewer predicted. Keying the set by extension removed the host lookup the test was asserting, so TestMIME_ZipContainerExtensionsResolve is replaced by TestMIME_ZipContainerDecisionIsHostIndependent, which iterates zipContainerExtensions and asserts the verdict directly — no OS MIME database involved. The host-dependence claim is additionally verified by running the suite in a golang:1.26-alpine container with no /etc/mime.types, which is now part of the recorded verification evidence.
status: addressed
---
