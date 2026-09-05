---
id: RR-L44IGF
type: review-response
title: No screen-reader announcement of the reset, and focus plan does not survive remounts
finding: The only live region in the form is the error summary (role="alert", :1993, gated on errorCount>0). A screen-reader user gets no signal that the record was created and the form cleared — in a workflow that benefits AT users more than mouse users. Add a role="status" announcing the created id and the reset. Separately, the planned single-nextTick focus is destroyed by the saveGeneration/fieldsRenderKey remount and by refreshStagedAffordances resolving later; focus after the last thing that can remount, or state that focus is best-effort.
severity: minor
status: addressed
resolution: >-
  New plan step 4b adds a role="status" announcement of the create + reset. Step 7 focus revised to run after the last remount and to be best-effort.
---
