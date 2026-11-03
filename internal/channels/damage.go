package channels

import (
	"log/slog"
	"sync"
	"time"
)

// DamageWarnInterval is how often a store warns again about one damaged
// channel that a timer listing keeps skipping.
const DamageWarnInterval = 10 * time.Minute

// maxDamageWarnKeys bounds the channels a [DamageWarner] remembers; past
// it the warner forgets them all and may warn about each once more.
const maxDamageWarnKeys = 1024

// DamageWarner logs a damaged channel that a listing skips, at most once
// per channel per [DamageWarnInterval]. A listing that runs every second
// would otherwise log the same damage every second. The zero value is
// ready to use.
type DamageWarner struct {
	mu   sync.Mutex
	last map[Key]time.Time
}

// Warn logs msg at warn level for channel k with err, unless it did so for
// k less than [DamageWarnInterval] before now.
func (w *DamageWarner) Warn(k Key, now time.Time, msg string, err error) {
	if !w.due(k, now) {
		return
	}
	slog.Warn(msg, "channel", k.String(), "error", err)
}

// due records a warning for k at now and reports whether one is due.
func (w *DamageWarner) due(k Key, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if last, ok := w.last[k]; ok && now.Sub(last) < DamageWarnInterval {
		return false
	}
	if w.last == nil || len(w.last) >= maxDamageWarnKeys {
		w.last = make(map[Key]time.Time)
	}
	w.last[k] = now
	return true
}
