---
id: RR-2PZB
type: review-response
title: Dry-run response after unmount writes to dead refs (benign but dead work)
finding: 'onBeforeUnmount calls stagedDryRunController?.abort(). If a dry-run is in flight when the user navigates away, the abort fires, but if the response had already arrived between resolve and the unmount handler running, refreshStagedAffordances writes to fieldAffordances/stagedVisibleProps refs on a destroyed component. Vue tolerates it, the controller !== stagedDryRunController check only catches superseding requests not unmount. Fix: an `unmounted` ref checked after the await; bail if true. Low value if Vue handles it cleanly today.'
severity: minor
resolution: Added module-scoped `stagedUnmounted` flag set true in onBeforeUnmount; refreshStagedAffordances checks it after the await (both success and catch paths) and bails before writing to refs. Dead work eliminated; benign behavior remains benign without relying on Vue tolerance.
status: addressed
---
