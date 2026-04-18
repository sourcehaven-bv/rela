package testutil

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Random value generators for test fixtures.
// These functions generate random but valid test data.
// Note: Builders should not be reused after Build() - each call creates a fresh builder.

var (
	rng   = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // weak RNG is acceptable for test fixtures
	rngMu sync.Mutex
)

// RandomString generates a random string like "word-a3f8".
func RandomString() string {
	return "word-" + randomSuffix()
}

// RandomID generates a random ID with the given prefix like "PREFIX-a3f8".
// If prefix is empty, generates just the suffix.
func RandomID(prefix string) string {
	suffix := randomSuffix()
	if prefix == "" {
		return suffix
	}
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

// RandomInt generates a random integer in the range [low, high].
// If low > high, the values are swapped.
func RandomInt(low, high int) int {
	if low > high {
		low, high = high, low
	}
	if low == high {
		return low
	}
	rngMu.Lock()
	defer rngMu.Unlock()
	return low + rng.Intn(high-low+1)
}

// RandomBool generates a random boolean.
func RandomBool() bool {
	rngMu.Lock()
	defer rngMu.Unlock()
	return rng.Intn(2) == 1
}

// RandomDate generates a random date within the last year.
func RandomDate() string {
	const daysInYear = 365
	now := time.Now()
	rngMu.Lock()
	daysAgo := rng.Intn(daysInYear)
	rngMu.Unlock()
	date := now.AddDate(0, 0, -daysAgo)
	return date.Format("2006-01-02")
}

// RandomEnumValue picks a random value from the given list.
// Panics if values is empty.
func RandomEnumValue(values []string) string {
	if len(values) == 0 {
		panic("RandomEnumValue: values cannot be empty")
	}
	rngMu.Lock()
	defer rngMu.Unlock()
	return values[rng.Intn(len(values))]
}

// randomSuffix generates a 4-character random suffix (base36).
func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 4)
	rngMu.Lock()
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	rngMu.Unlock()
	return string(b)
}
