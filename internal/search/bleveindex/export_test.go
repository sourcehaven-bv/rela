package bleveindex

// SetFormat overwrites the stored format version; an empty value removes it,
// so the index looks like one written before versioning.
func SetFormat(idx *Index, v string) error {
	if v == "" {
		return idx.index.DeleteInternal(formatKey)
	}
	return idx.index.SetInternal(formatKey, []byte(v))
}
