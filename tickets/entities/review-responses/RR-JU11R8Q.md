---
id: RR-JU11R8Q
type: review-response
title: 'Design: database roles for provisioner and server'
finding: If the provisioner connects as another role the server cannot write the tables it creates; the server must not create rela_control.
severity: minor
resolution: The provisioner and server connect with the same RELA_DATABASE_URL role; only the Provisioner creates rela_control; the guide says so.
status: addressed
---
