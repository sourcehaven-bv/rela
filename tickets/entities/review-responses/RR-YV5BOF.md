---
id: RR-YV5BOF
type: review-response
title: Golden test is memstore-only and lacks faces and a hidden-only cell
finding: The golden test cannot show identical output on the other backends. It had no faced row although the change reroutes the face-ownership input and its doc did not mention the self-loop or the hidden-only cell.
severity: minor
resolution: The doc now names the self-loop and TKT-3's hidden-only cell and states memstore-only coverage with the reason order holds elsewhere. Added TestExport_List_FacedRowCarriesOnlyItsOwnEdges. The 550-row postgres demo export was byte-identical before and after.
status: addressed
---
