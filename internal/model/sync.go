package model

// SyncResult contains statistics from a sync operation.
type SyncResult struct {
	EntitiesLoaded  int
	RelationsLoaded int
	Errors          []error
}

// SyncError represents an error during sync.
type SyncError struct {
	File    string
	Message string
}

func (e *SyncError) Error() string {
	return e.File + ": " + e.Message
}
