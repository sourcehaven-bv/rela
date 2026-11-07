package pgexec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The adapter's statements run in the pgflowrun and pgchannels suites,
// against a live database.

func TestNewRejectsNil(t *testing.T) {
	_, err := New(nil)
	require.Error(t, err)
}

func TestRebind(t *testing.T) {
	require.Equal(t, "SELECT 1", rebind("SELECT 1"))
	require.Equal(t, "a = $1 AND b IN ($2, $3)", rebind("a = ? AND b IN (?, ?)"))
	require.Equal(t, "x = $1 -- é", rebind("x = ? -- é"))
}
