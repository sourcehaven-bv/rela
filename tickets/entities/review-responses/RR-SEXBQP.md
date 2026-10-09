---
id: RR-SEXBQP
type: review-response
title: Foreign push evicts owner items and fills their pile quota
finding: '[security] A push into another user''s pile (Lua/MCP/automation with owner) goes through the same AddItems(MaxItems) eviction and CreatePile cap as the owner. One MCP call with 500 readable ids wipes a victim''s hand-collected pile (answer is always added:0); 50 calls with create:true exhaust their 50-pile cap so their own POST /_piles gets pile_limit. internal/piles/service.go Push/pushTo; default create=true in lua/piles.go and mcp/tools_piles.go.'
severity: significant
resolution: Store.AddItems takes an Overflow mode; a foreign push uses KeepExisting (never evicts; fills only free room atomically in kv and pg) and its create is capped at MaxForeignPiles=25 so the owner always keeps 25 slots. Both are silent no-ops past the limit. Foreign pushes are logged at Info without the pile name. pilestest RunKeepExistingTests, TestService_ForeignPushNeverEvicts, TestService_ForeignCreatesStopAtMaxForeignPiles; guides updated.
status: addressed
---
