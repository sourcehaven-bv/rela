---
from: TKT-5V8704
relation: depends-on
to: TKT-8VVBRI
---

PR 2 restructures the same three `.properties-list` components that carry 19
radius/font literals (SectionEditForm 3, PropertyDisplay 2, SidePanel 14).
Landing tokens first means the layout PR consumes them rather than churning the
same declarations twice and creating a needless merge conflict.
