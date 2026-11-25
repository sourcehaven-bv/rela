package dataentryconfig

import "fmt"

// ListOrderManual is the `order:` of a list whose rows each user may drag
// into their own order (TKT-AVNEX5T). The order is stored per user outside
// the graph; membership stays computed by the list.
const ListOrderManual = "manual"

// checkListOrder checks a list's `order:`. Only `manual` is accepted. A
// grouped list keeps its stored order inside each run of equal group
// values, which needs the group to be a plain property value: a relation
// group or date buckets would put rows with different values in one
// section.
func checkListOrder(listID string, l List) []string {
	switch l.Order {
	case "":
		return nil
	case ListOrderManual:
	default:
		return []string{fmt.Sprintf("list %q has invalid order %q (valid: %s)", listID, l.Order, ListOrderManual)}
	}
	if g := l.GroupBy; g != nil && (g.Relation != "" || g.Buckets != nil) {
		return []string{fmt.Sprintf(
			"list %q: order: manual needs group_by on a property without buckets", listID)}
	}
	return nil
}
