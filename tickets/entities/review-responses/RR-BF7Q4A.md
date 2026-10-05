---
id: RR-BF7Q4A
type: review-response
title: FaceSet.QueryFaces widens to every face for the empty set
finding: '[security] QueryFaces returned nil for the empty set, which a store reads as every face; only caller discipline (admit stops on IsNone) prevented a widened read.'
severity: nit
resolution: QueryFaces now returns (faces, ok) and reports ok=false for the empty set; loadInWorld misses without querying. TestFaceSet pins all three states.
status: addressed
---
