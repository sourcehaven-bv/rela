---
id: RR-ZDK32
type: review-response
title: Helper must handle empty entity.type (don't render <a href='/entity//id'>)
finding: If entity.type is empty (which the plan acknowledges as an edge case), helper would emit /entity//some-id -> server 404. Either guarantee entity.type is always non-empty (and unit test the contract), or have helper return an empty string AND have template guard with v-if='href'.
severity: minor
resolution: Helper returns empty string when entity.type is empty (avoids malformed /entity//id). Templates guard with v-if="href" and skip rendering an anchor in that case. Unit test asserts both contract and rendering path.
status: addressed
---
