package channels

import "fmt"

// MaxGetMessages is the most refs one [Store.GetMessages] call may name. It
// equals the inbox's page limit, so one inbox page needs one call.
const MaxGetMessages = 200

// MessageRef names one message: its channel and its seq.
type MessageRef struct {
	Key Key
	Seq int64
}

// CheckGetMessages validates the refs of [Store.GetMessages]: at most
// [MaxGetMessages] refs, each with a valid key and a seq above 0, else
// [ErrInvalidKey].
func CheckGetMessages(refs []MessageRef) error {
	if len(refs) > MaxGetMessages {
		return fmt.Errorf("%w: %d message refs, over %d", ErrInvalidKey, len(refs), MaxGetMessages)
	}
	for _, r := range refs {
		if err := r.Key.Validate(); err != nil {
			return err
		}
		if r.Seq <= 0 {
			return fmt.Errorf("%w: message %d in %s", ErrInvalidKey, r.Seq, r.Key)
		}
	}
	return nil
}

// GroupRefs returns the distinct refs grouped by channel, each group's seqs
// in the order first named, and the channels in the order first named.
func GroupRefs(refs []MessageRef) (keys []Key, seqs map[Key][]int64) {
	seqs = make(map[Key][]int64)
	seen := make(map[MessageRef]bool, len(refs))
	for _, r := range refs {
		if seen[r] {
			continue
		}
		seen[r] = true
		if _, ok := seqs[r.Key]; !ok {
			keys = append(keys, r.Key)
		}
		seqs[r.Key] = append(seqs[r.Key], r.Seq)
	}
	return keys, seqs
}
