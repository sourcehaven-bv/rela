---
id: RR-J0WH5T0
type: review-response
title: JSON-number writes stored as float64
finding: Dataentry and MCP decode JSON without UseNumber and nothing converted a decimal to text; a number was stored as a float and the stored type depended on the client.
severity: significant
resolution: createCore and updateCore now store each decimal value that arrives as a number as its decimal text (storeDecimalsAsText; TestStoreDecimalsAsText). GUIDE-metamodel says to send decimals as strings and that a number past about 15 significant digits has already lost digits.
status: addressed
---
