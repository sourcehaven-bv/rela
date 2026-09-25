---
id: RR-YVM1X3
type: review-response
title: WithLuaTools not validated against its deps
finding: A server built WithLuaTools with zero LuaWriteDeps failed at request time.
severity: minor
resolution: NewServer and ReloadDeps refuse WithLuaTools without LuaWriteDeps.EntityManager. Pinned in TestNewServer_LuaToolsAreOptIn.
status: addressed
---
