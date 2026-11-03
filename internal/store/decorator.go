package store

// Decorator is a store that wraps another and passes its calls through, as
// the test recorders in storetest do. A lookup keyed on the concrete backend
// type, such as the database handle a service shares with the store, goes
// through [Innermost] so a decorated backend still answers it.
type Decorator interface {
	// Unwrap returns the wrapped store.
	Unwrap() Store
}

// Innermost returns st with every [Decorator] layer removed.
//
// Nil: accepted; a nil st is returned as is.
func Innermost(st Store) Store {
	for {
		d, ok := st.(Decorator)
		if !ok {
			return st
		}
		st = d.Unwrap()
	}
}
