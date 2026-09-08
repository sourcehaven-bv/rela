---
from: BUG-Y0GNSB
type: affects
to: authorization
---

The defect is in how the ACL write subject is assembled: a security-relevant
dimension (face) was present on the loaded entity but omitted from the subject,
so the decision was made about a different resource than the one written.
