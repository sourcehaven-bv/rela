package lua

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	glua "github.com/yuin/gopher-lua"
)

// Constants bounding the cache's resource use. See ticket TKT-V8UQC.
const (
	// maxCacheKeyBytes is the limit for a single Lua-supplied cache key
	// (the user key, not the namespaced key). Keeps namespaced keys
	// bounded and prevents abuse.
	maxCacheKeyBytes = 512

	// maxCacheEntries is the global cap across all namespaces. LRU
	// eviction kicks in when a new entry would exceed this.
	maxCacheEntries = 10000

	// defaultCacheTTL is applied when the caller does not specify a
	// ttl option. One hour strikes a balance for scheduled tasks
	// re-running on short intervals.
	defaultCacheTTL = time.Hour

	// hashFieldHexLen is the width in hex characters of the diagnostic
	// fields emitted to logs AND of the fixed-width namespace prefix in
	// stored keys. fnv-1a 64-bit gives 16 hex chars. Crypto strength
	// isn't needed here — we're not storing user data, comparing
	// user-supplied input against trusted values, or guarding anything
	// with these; fnv is ~10× faster than sha256 and allocation-free.
	//
	// Collision risk: namespaces are hashed to this width before
	// concatenation with the user key, which means two paths that fnv
	// collide would share a cache namespace. Probability for N scripts
	// sharing 2^64 space is ~N²/2^65, negligible for any realistic N.
	hashFieldHexLen = 16

	// maxCacheTTLSeconds is the largest accepted TTL (in seconds) we
	// can convert to a time.Duration without float64→int64 overflow.
	// math.MaxInt64 / 1e9 leaves just under 292 years, which is orders
	// of magnitude more than any legitimate cache use.
	maxCacheTTLSeconds = float64(math.MaxInt64) / float64(time.Second)
)

// Cache is an in-memory key/value store with TTL and global LRU
// eviction, shared across all Lua runtimes in a single process. It is
// safe for concurrent use. Callers inject a shared Cache into each
// lua.Runtime via WithCache; runtimes namespace their operations by the
// script path (see Runtime.scriptPath) so two scripts that happen to
// pick the same user key do not collide.
//
// Not persisted: each process starts with an empty cache. For cross-
// process durability see ticket TKT-135Q (the AI response disk cache).
//
// Concurrency: uses a plain sync.Mutex (not RWMutex). Every hit path
// mutates lastAccess for LRU accounting, so a reader-writer split
// would require two-phase locking (RLock lookup, Lock upgrade for
// lastAccess) without a clear win. Misses, nil-deletes, and TTL-expiry
// deletes do not touch lastAccess — under heavy read-miss contention a
// switch to RWMutex (with atomic lastAccess) is a defensible
// optimization, but needs a benchmark before the complexity is worth it.
//
// The zero value is not usable; construct with NewCache.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry

	// now returns the current time. Tests may override via
	// (*Cache).SetNow to avoid time.Sleep in TTL/LRU assertions.
	now func() time.Time
}

type cacheEntry struct {
	// values is the sequence of values the memoizer captured (or a
	// single-element slice produced by set). Stored as []interface{} so
	// round-tripping through GoToLuaValue yields the same Lua shape.
	values []interface{}

	// expiresAt is the absolute time at which this entry becomes
	// unreadable. A zero value means "never expires" (still subject to
	// LRU).
	expiresAt time.Time

	// lastAccess is updated on every successful read AND on write. It
	// drives LRU eviction: the entry with the smallest lastAccess goes
	// first.
	lastAccess time.Time
}

// NewCache builds a ready-to-use Cache. The returned value is a singleton
// in the logical sense — callers typically construct one per process and
// pass it to every lua.Runtime they build.
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*cacheEntry),
		now:     time.Now,
	}
}

// SetNow overrides the Cache's time source. Intended for tests that need
// deterministic TTL/LRU behavior without time.Sleep. Production code
// should never call this.
//
// INVARIANT: the replacement function MUST be safe to call under c.mu
// and is exclusively invoked from locked methods. Adding a lock-free
// caller of c.now (e.g. a metrics accessor) would create a data race
// with any closure-based fake clock that mutates a shared counter.
// If you need such an accessor, snapshot c.now under the lock first.
func (c *Cache) SetNow(f func() time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = f
}

// get returns the stored values for key, or (nil, false) on miss. The
// entry is removed lazily if its TTL has passed. lastAccess is updated on
// a hit so LRU eviction orders entries by real access time, not just
// insert time.
func (c *Cache) get(key string) ([]interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	now := c.now()
	if !e.expiresAt.IsZero() && !now.Before(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	e.lastAccess = now
	// Return a shallow copy so callers can't mutate our internal slice.
	out := make([]interface{}, len(e.values))
	copy(out, e.values)
	return out, true
}

// set stores values under key with the given TTL. A ttl of zero or
// negative means "never expires". When the cache is at capacity, the
// least-recently-accessed entry is evicted.
func (c *Cache) set(key string, values []interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}
	// Only evict if adding a new key would exceed the cap. Replacing an
	// existing key doesn't grow the cache.
	if _, replacing := c.entries[key]; !replacing && len(c.entries) >= maxCacheEntries {
		c.evictLRULocked()
	}
	// Copy the values so later external mutation can't affect cached
	// state. (luaValueToGo already allocates fresh maps/slices; the
	// wrapper slice is ours too.)
	stored := make([]interface{}, len(values))
	copy(stored, values)
	c.entries[key] = &cacheEntry{
		values:     stored,
		expiresAt:  expiresAt,
		lastAccess: now,
	}
}

// delete removes an entry unconditionally. Safe if the key is absent.
func (c *Cache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// evictLRULocked walks the entries map once to find the entry with the
// smallest lastAccess and removes it. Must be called with c.mu held.
// With maxCacheEntries = 10,000 the walk is microseconds; swap for a
// heap if profiling shows it.
func (c *Cache) evictLRULocked() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, e := range c.entries {
		if first || e.lastAccess.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.lastAccess
			first = false
		}
	}
	if first {
		return
	}
	delete(c.entries, oldestKey)
	slog.Debug("cache evict", "cache", "evict", "key_hash", hashKey(oldestKey))
}

// hashKey returns a short hex digest of s, suitable for diagnostic
// logging. Never log the raw key — it may contain user data (entity
// properties, AI prompts, paths).
//
// Uses fnv-1a 64-bit because the use case is "make a stable identifier
// for log grep," not "resist pre-image attacks." Same 16-hex-char
// width as the sha256[:16] it replaces, so existing grep patterns
// keep working.
func hashKey(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%0*x", hashFieldHexLen, h.Sum64())
}

// unnamespacedMarker is the literal value emitted for namespace_hash
// when a cache key is shorter than the fixed-width namespace prefix
// (shouldn't happen via the Lua bindings, but might via a direct Go
// call with a hand-built key). A stable literal surfaces the anomaly
// to log operators instead of shipping a truncated hex string that
// looks plausible.
const unnamespacedMarker = "<unnamespaced>"

// logFields builds the (namespace_hash, key_hash) pair for a cache log
// line from a fully-namespaced cache key. Safe to call with any string
// — never emits the raw user key.
//
// Keys are `hashKey(scriptPath) + userKey` with a fixed-width prefix,
// so the namespace half is a slice (no scan, no re-hash). The user
// half is hashed for the log field — we do not log raw user keys,
// they may contain entity properties, AI prompts, or paths.
func logFields(event, namespacedKey string) []any {
	nsHash := unnamespacedMarker
	userHash := hashKey(namespacedKey)
	if len(namespacedKey) >= hashFieldHexLen {
		nsHash = namespacedKey[:hashFieldHexLen]
		userHash = hashKey(namespacedKey[hashFieldHexLen:])
	}
	return []any{
		"cache", event,
		"namespace_hash", nsHash,
		"key_hash", userHash,
	}
}

// registerCacheBindings installs rela.cache.{get,set,memoize} on the
// supplied rela table. It is a no-op if the runtime has no cache wired.
// Each binding guards against the inline/eval case (no scriptPath) by
// raising a Lua error with a fixed message, so accidental cache use in
// lua_eval surfaces loudly instead of silently bleeding state across
// sessions.
func (r *Runtime) registerCacheBindings(rela *glua.LTable) {
	if r.cache == nil {
		return
	}
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "get", r.L.NewFunction(r.luaCacheGet))
	r.L.SetField(tbl, "set", r.L.NewFunction(r.luaCacheSet))
	r.L.SetField(tbl, "memoize", r.L.NewFunction(r.luaCacheMemoize))
	r.L.SetField(rela, "cache", tbl)
}

// requireCacheContext is called at the top of every cache binding. It
// enforces the "no cache outside of RunFile" rule by raising a Lua
// error when scriptPath is unset. Returns the namespaced key for the
// given user key, and validates its length.
//
// The namespaced-key format is a fixed-width hex hash of the script
// path followed by the raw user key, with no separator. That lets
// splitNamespaced / logFields reconstruct the namespace hash with
// O(1) slicing instead of scanning for a separator byte, and removes
// any reliance on the user key not containing the separator. Raw
// script paths are never stored in cache keys (they'd leak structure
// on any memory dump).
func (r *Runtime) requireCacheContext(ls *glua.LState, userKey string) string {
	if r.scriptPath == "" {
		ls.RaiseError("cache: not available in inline/eval contexts")
		return ""
	}
	if len(userKey) > maxCacheKeyBytes {
		ls.RaiseError("cache: key length %d exceeds limit %d",
			len(userKey), maxCacheKeyBytes)
		return ""
	}
	return hashKey(r.scriptPath) + userKey
}

// cacheOptions captures the parsed options table for set/memoize.
// Bool fields default to false; ttl of zero means "use the caller's
// default" (defaultCacheTTL) and zeroTTL distinguishes "not specified"
// from "explicitly requested no expiry".
type cacheOptions struct {
	ttl    time.Duration
	hasTTL bool
	bypass bool
}

// parseCacheOptions reads an optional options table at argument idx on
// the Lua stack. It enforces an allowlist of keys so typos like
// `refersh` raise loudly instead of being silently dropped. The caller
// passes the set of allowed option names.
func parseCacheOptions(ls *glua.LState, idx int, allowed map[string]bool) cacheOptions {
	var opts cacheOptions
	if ls.GetTop() < idx {
		return opts
	}
	val := ls.Get(idx)
	if val == glua.LNil {
		return opts
	}
	tbl, ok := val.(*glua.LTable)
	if !ok {
		ls.RaiseError("cache: options must be a table")
		return opts
	}
	tbl.ForEach(func(k, v glua.LValue) {
		ks, kok := k.(glua.LString)
		if !kok {
			ls.RaiseError("cache: option keys must be strings")
			return
		}
		name := string(ks)
		if !allowed[name] {
			ls.RaiseError("cache: unknown option %q; recognized: %s",
				name, allowedOptionList(allowed))
			return
		}
		switch name {
		case "ttl":
			n, nok := v.(glua.LNumber)
			if !nok {
				ls.RaiseError("cache: option ttl must be a number")
				return
			}
			opts.hasTTL = true
			f := float64(n)
			if f <= 0 {
				opts.ttl = 0 // "no expiry"
				return
			}
			// Guard against float64→int64 overflow. The naive conversion
			// `time.Duration(f * 1e9)` wraps to a large negative duration
			// for TTLs above ~292 years, so the entry is born already
			// expired and the next get is a silent miss. Reject loudly
			// so the script author knows their TTL was bogus.
			if f > maxCacheTTLSeconds {
				// maxCacheTTLSeconds is ~9.2e18, which fits in int64
				// after runtime conversion (not as a constant).
				maxInt := int64(math.Floor(maxCacheTTLSeconds))
				ls.RaiseError("cache: ttl too large (max %d seconds)", maxInt)
				return
			}
			opts.ttl = time.Duration(f * float64(time.Second))
		case "bypass":
			b, bok := v.(glua.LBool)
			if !bok {
				ls.RaiseError("cache: option bypass must be a boolean")
				return
			}
			opts.bypass = bool(b)
		}
	})
	return opts
}

// allowedOptionList renders the allowlist as a stable comma-joined
// string for error messages. Sorted lexicographically so the output is
// deterministic across runs (map iteration is not).
func allowedOptionList(allowed map[string]bool) string {
	keys := make([]string, 0, len(allowed))
	for k := range allowed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// ttlOrDefault picks the effective TTL for a write. An unspecified
// option (hasTTL=false) becomes defaultCacheTTL; an explicit zero or
// negative stays as zero (never expires).
func (opts cacheOptions) ttlOrDefault() time.Duration {
	if !opts.hasTTL {
		return defaultCacheTTL
	}
	return opts.ttl
}

// getAllowedOpts / setAllowedOpts / memoizeAllowedOpts — small
// package-level singletons so each parse call reuses the same allowlist
// without rebuilding it.
var (
	setAllowedOpts     = map[string]bool{"ttl": true}
	memoizeAllowedOpts = map[string]bool{"ttl": true, "bypass": true}
)

// luaCacheGet implements rela.cache.get(key) -> value|nil. It accepts no
// options; a second argument raises a Lua error to prevent the
// "user passed opts expecting them to do something" footgun.
func (r *Runtime) luaCacheGet(ls *glua.LState) int {
	userKey := ls.CheckString(1)
	// Hard-reject any second argument: get has no options.
	if ls.GetTop() > 1 && ls.Get(2) != glua.LNil {
		ls.RaiseError("cache.get: takes 1 argument (key); no options accepted")
		return 0
	}
	namespacedKey := r.requireCacheContext(ls, userKey)
	values, ok := r.cache.get(namespacedKey)
	if !ok {
		slog.Debug("cache miss", logFields("miss", namespacedKey)...)
		ls.Push(glua.LNil)
		return 1
	}
	slog.Debug("cache hit", logFields("hit", namespacedKey)...)
	// get-semantic: if exactly one value stored, return it directly;
	// otherwise push them all (for symmetry with memoize multi-return).
	for _, v := range values {
		ls.Push(GoToLuaValue(ls, v))
	}
	if len(values) == 0 {
		// Degenerate case: memoize-stored empty tuple. Push nil so the
		// caller distinguishes from a miss only via downstream logic —
		// but a caller that uses `get` on a memoize-zero-value entry is
		// already in strange territory.
		ls.Push(glua.LNil)
		return 1
	}
	return len(values)
}

// luaCacheSet implements rela.cache.set(key, value, opts?).
// Passing nil as the value deletes the entry. Rejects unrepresentable
// Lua values (functions, userdata, coroutines) at API time, so scripts
// cannot accidentally poison the cache with a handle that will
// eventually be stale.
func (r *Runtime) luaCacheSet(ls *glua.LState) int {
	userKey := ls.CheckString(1)
	valArg := ls.Get(2)
	opts := parseCacheOptions(ls, 3, setAllowedOpts)

	namespacedKey := r.requireCacheContext(ls, userKey)

	// Nil value -> delete. This matches Lua table semantics ("nil means
	// absent") and avoids a separate rela.cache.delete surface.
	if valArg == glua.LNil {
		r.cache.delete(namespacedKey)
		slog.Debug("cache delete", logFields("delete", namespacedKey)...)
		return 0
	}

	// Reject unrepresentable values before storing. The walk recurses
	// into tables so a function nested inside a map still surfaces.
	if err := validateRepresentable(valArg); err != nil {
		ls.RaiseError("cache.set: %s", err.Error())
		return 0
	}

	r.cache.set(namespacedKey, []interface{}{luaValueToGo(valArg)}, opts.ttlOrDefault())
	slog.Debug("cache store", logFields("store", namespacedKey)...)
	return 0
}

// luaCacheMemoize implements rela.cache.memoize(key, fn, opts?). On hit
// the cached values are returned; on miss, fn is called, all its return
// values are captured (matching Lua's natural multi-return convention),
// stored, and emitted.
//
// The cache mutex is released across fn's execution so that (a) a
// long-running fn does not block unrelated readers, (b) fn may
// legitimately call rela.cache.{get,set,memoize} without deadlocking.
// Two concurrent misses on the same key will both execute fn; this is
// acceptable for a cache (values should be deterministic) and strictly
// preferable to holding a lock across arbitrary script code.
//
// FOOTGUN — last write wins: if fn itself calls `rela.cache.set(key, …)`
// with the SAME key, memoize then overwrites that write with its own
// captured return values. Scripts that want side-channel writes
// alongside memoization must use a different key (e.g. `key .. ":aux"`).
// Detecting and skipping the overwrite would require versioning every
// entry, and is not worth the complexity for an uncommon pattern.
func (r *Runtime) luaCacheMemoize(ls *glua.LState) int {
	userKey := ls.CheckString(1)
	fn := ls.CheckFunction(2)
	opts := parseCacheOptions(ls, 3, memoizeAllowedOpts)

	namespacedKey := r.requireCacheContext(ls, userKey)

	// Check cache unless explicitly bypassed.
	if !opts.bypass {
		if values, ok := r.cache.get(namespacedKey); ok {
			slog.Debug("cache hit", logFields("hit", namespacedKey)...)
			for _, v := range values {
				ls.Push(GoToLuaValue(ls, v))
			}
			return len(values)
		}
		slog.Debug("cache miss", logFields("miss", namespacedKey)...)
	}

	// Miss (or bypass) — invoke fn, capturing ALL return values via
	// stack delta. This is the same pattern RunActionString uses:
	// record top before the call, compare after.
	topBefore := ls.GetTop()
	ls.Push(fn)
	if err := ls.PCall(0, glua.MultRet, nil); err != nil {
		// Propagate as a Lua error; do NOT cache a failure.
		// Use the error message as-is — it comes from gopher-lua and
		// may include the user's own error string (never the cache key).
		ls.RaiseError("cache.memoize: %s", err.Error())
		return 0
	}
	topAfter := ls.GetTop()
	nRet := topAfter - topBefore

	// Two-pass so we either reject all or convert all. Mixing validation
	// with conversion would be fragile if luaValueToGo ever grew side
	// effects, and the test author should see a full error for any
	// one rejected return without any other values having been
	// processed first. PCall pushed returns at topBefore+1 .. topAfter.
	for i := range nRet {
		if err := validateRepresentable(ls.Get(topBefore + 1 + i)); err != nil {
			ls.RaiseError("cache.memoize: fn return value %d: %s", i+1, err.Error())
			return 0
		}
	}
	goValues := make([]interface{}, nRet)
	for i := range nRet {
		goValues[i] = luaValueToGo(ls.Get(topBefore + 1 + i))
	}

	r.cache.set(namespacedKey, goValues, opts.ttlOrDefault())
	slog.Debug("cache store", logFields("store", namespacedKey)...)

	// The returns are already on the stack; just report the count.
	return nRet
}

// validateRepresentable walks a Lua value rejecting any node that the
// cache cannot safely store. The cache holds plain data; functions,
// userdata, channels, and coroutines are stateful or hold live
// resources, so caching them is almost certainly a bug.
//
// Cyclic tables are rejected too — caching a self-referential structure
// would loop forever in luaValueToGo. Without this guard the recursion
// eats the Go stack and the *entire process* terminates with "goroutine
// stack exceeds 1000000000-byte limit"; that's unrecoverable even from
// a PCall, so a Lua error at the cache boundary is the only safe option.
func validateRepresentable(lv glua.LValue) error {
	return validateRepresentableSeen(lv, make(map[*glua.LTable]bool))
}

func validateRepresentableSeen(lv glua.LValue, seen map[*glua.LTable]bool) error {
	switch v := lv.(type) {
	case glua.LBool, glua.LNumber, glua.LString, *glua.LNilType:
		return nil
	case *glua.LTable:
		if seen[v] {
			return errors.New("cannot cache cyclic table")
		}
		seen[v] = true
		var err error
		v.ForEach(func(k, val glua.LValue) {
			if err != nil {
				return
			}
			// Table keys must also be representable — an integer/string
			// key is fine, a function key is not.
			if e := validateRepresentableSeen(k, seen); e != nil {
				err = fmt.Errorf("table key: %w", e)
				return
			}
			if e := validateRepresentableSeen(val, seen); e != nil {
				err = e
			}
		})
		// Unmark so sibling subtrees that share a (non-cyclic) table
		// reference don't get false-positive rejected. Cyclic detection
		// is per-ancestry-chain, not per-occurrence.
		delete(seen, v)
		return err
	default:
		return fmt.Errorf("cannot cache value of type %s", lv.Type().String())
	}
}
