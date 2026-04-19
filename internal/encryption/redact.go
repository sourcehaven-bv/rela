package encryption

import "fmt"

// safe renders a byte slice for inclusion in error messages without
// revealing its content. Use this anywhere a []byte might otherwise be
// interpolated via %v / %x / string(...) in an error.
func safe(b []byte) string {
	return fmt.Sprintf("<%d bytes>", len(b))
}
