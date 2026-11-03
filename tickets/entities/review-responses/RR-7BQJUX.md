---
id: RR-7BQJUX
type: review-response
title: Schema hot reload keeps the old fsstore layout, so new types become unwritable
finding: 'fileLayout.schemas is fixed in fsstore.New and never refreshed. MCP schema hot reload (cli/mcp_wiring.go mcpServices.reload, ForReassembly) deliberately reuses the open store (TestMCPServices_ReloadReusesStoreAndSearcher). After this change, a type added to schema.yaml mid-session passes entitymanager but fsstore rejects it until restart. That breaks TKT-NU247U, and TestMCPServices_ReloadPicksUpNewType (default fs build) will fail. The plan''s edge case ''if the layout''s schemas are refreshed'' assumes a refresh that does not exist. Add a schema-refresh seam: hold the layout in an atomic.Pointer and add a narrow method such as SetSchemas that the reassembly path calls when it reuses the store. Rescan the plural directories that became declared. This also fixes a bug present today: a hot-added type with a custom plural is written to ''<type>s'' and lost on the next open. Add an AC and a test for reload followed by create.'
severity: critical
resolution: 'Plan updated: FSStore.SetSchemas swaps the layout under the write lock and scans newly declared type directories; the schema hot-reload site calls it through a consumer-side seam supplied by the fs wiring (AC7, including TestMCPServices_ReloadPicksUpNewType creating an entity of the new type).'
status: addressed
---
