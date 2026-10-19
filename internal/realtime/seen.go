package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// The seen-set is what the trigger engine knows of each rule's matches,
// kept in the [KV] so a restart, and every process sharing the KV, carries
// on where the last round stopped. Events say only which entity changed,
// never what it was before, so the set is what makes leave and change
// detectable.
//
// Each rule's entries are split over shards, keyed by entity id, so a
// round reads and writes only the shards of the ids it handles. The index
// lists every rule with its shard count, because the KV cannot list keys:
// without it the shards of a removed rule could not be found to delete.
// A shard key carries the shard count, so a rule split again writes its new
// shards beside the old ones and switches over by saving the index.
const (
	seenPrefix   = "realtime/seen/"
	seenIndexKey = seenPrefix + "index"
)

// seenIndex lists the rules that have a seen-set.
type seenIndex struct {
	Rules map[string]seenRule `json:"rules"`
}

// seenRule is one rule's entry in the index.
type seenRule struct {
	Shards int `json:"shards"`
}

// seenShard maps an entity ref (an id, or id@face for a rule that reads
// each face) to the fingerprint of the properties the rule read when it
// last matched. An entity that does not match has no entry.
type seenShard map[string]string

// seenStore reads and writes the seen-set in a [KV].
type seenStore struct {
	kv KV
}

func isMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}

func (s seenStore) loadIndex(ctx context.Context) (seenIndex, error) {
	idx := seenIndex{Rules: map[string]seenRule{}}
	data, err := s.kv.Get(ctx, seenIndexKey)
	if isMissing(err) {
		return idx, nil
	}
	if err != nil {
		return idx, fmt.Errorf("realtime: read the trigger index: %w", err)
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return seenIndex{Rules: map[string]seenRule{}}, fmt.Errorf("realtime: read the trigger index: %w", err)
	}
	if idx.Rules == nil {
		idx.Rules = map[string]seenRule{}
	}
	return idx, nil
}

func (s seenStore) saveIndex(ctx context.Context, idx seenIndex) error {
	data, err := json.Marshal(idx)
	if err != nil { // coverage-ignore: unreachable: the index always marshals
		return err
	}
	if err := s.kv.Put(ctx, seenIndexKey, data); err != nil {
		return fmt.Errorf("realtime: save the trigger index: %w", err)
	}
	return nil
}

// shardKey is the KV key of shard n of a rule split over shards.
func shardKey(ruleID string, shards, n int) string {
	return seenPrefix + ruleID + "/" + strconv.Itoa(shards) + "-" + strconv.Itoa(n)
}

func (s seenStore) loadShard(ctx context.Context, key string) (seenShard, error) {
	data, err := s.kv.Get(ctx, key)
	if isMissing(err) {
		return seenShard{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("realtime: read %s: %w", key, err)
	}
	out := seenShard{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("realtime: read %s: %w", key, err)
	}
	return out, nil
}

// saveShard writes sh at key, or deletes the key when sh is empty.
func (s seenStore) saveShard(ctx context.Context, key string, sh seenShard) error {
	if len(sh) == 0 {
		return s.kv.Delete(ctx, key)
	}
	data, err := json.Marshal(sh)
	if err != nil { // coverage-ignore: unreachable: a map of strings always marshals
		return err
	}
	return s.kv.Put(ctx, key, data)
}

// dropShards deletes every shard of a rule split over shards.
func (s seenStore) dropShards(ctx context.Context, ruleID string, shards int) error {
	var errs []error
	for n := range shards {
		if err := s.kv.Delete(ctx, shardKey(ruleID, shards, n)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// shardOf is the shard that holds every entry of entity id. All faces of an
// entity share one, so a round finds them with one read.
func shardOf(id string, shards int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return int(h.Sum32() % uint32(shards)) //nolint:gosec // shards is at most realtimelimits.MaxSeenShards
}

// shardsFor is the shard count for n entries: a power of two that keeps a
// shard near [realtimelimits.SeenPerShard] entries, at most
// [realtimelimits.MaxSeenShards].
func shardsFor(n int) int {
	shards := 1
	for shards < realtimelimits.MaxSeenShards && n > shards*realtimelimits.SeenPerShard {
		shards *= 2
	}
	return shards
}

// fingerprint identifies the values of fields in props. It is a hash, so
// the seen-set never holds a property value.
func fingerprint(fields []string, props map[string]any) string {
	vals := make(map[string]any, len(fields))
	for _, f := range fields {
		vals[f] = props[f]
	}
	data, err := json.Marshal(vals)
	if err != nil { // coverage-ignore: unreachable: store values always marshal
		data = fmt.Append(nil, vals)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}
