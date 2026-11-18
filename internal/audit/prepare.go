package audit

// Prepare returns rec in the form a persistent sink stores it: Source
// filled in ([SourceOf]) and every string field sanitized, as
// [Filesystem] writes it. A sink outside this package calls it so the
// stored record matches the JSONL one field for field.
func Prepare(rec Record) Record {
	return sanitize(withSource(rec))
}
