package lua

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	glua "github.com/yuin/gopher-lua"
)

// newCachedWriter builds a writer runtime with a fresh Cache and the
// given script path already set. Most tests want this shape — a minimal
// runtime with the cache wired and a stable namespace.
func newCachedWriter(t *testing.T, scriptPath string) (*Runtime, *Cache) {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	c := NewCache()
	r := NewWriter(ws.services("/tmp"), &buf, WithCache(c))
	r.SetScriptPath(scriptPath)
	t.Cleanup(r.Close)
	return r, c
}

// mustRun executes code and fails the test on error. Kept small so test
// bodies stay readable — one-liner checks instead of repeated boilerplate.
func mustRun(t *testing.T, r *Runtime, code string) {
	t.Helper()
	if err := r.RunString(code); err != nil {
		t.Fatalf("RunString failed: %v\ncode:\n%s", err, code)
	}
}

// mustRunErr executes code, expects an error, and returns it. Fails if
// the code runs cleanly (the test asserted against the wrong behavior).
func mustRunErr(t *testing.T, r *Runtime, code string) error {
	t.Helper()
	err := r.RunString(code)
	if err == nil {
		t.Fatalf("expected error running:\n%s", code)
	}
	return err
}

func TestCacheUnregisteredWithoutOption(t *testing.T) {
	// Without WithCache, rela.cache.* must not exist so callers don't
	// accidentally use an uninitialised cache. The Lua VM itself raises
	// "attempt to call a nil value" on undefined bindings.
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()
	r.SetScriptPath("foo.lua")
	err := r.RunString(`rela.cache.get("k")`)
	if err == nil {
		t.Fatal("expected error — rela.cache should not be registered without WithCache")
	}
	if !strings.Contains(err.Error(), "nil value") &&
		!strings.Contains(err.Error(), "attempt to index") {

		t.Fatalf("unexpected error shape: %v", err)
	}
}

func TestCacheInInlineRaisesError(t *testing.T) {
	// No script path => inline/eval context. rela.cache.* must raise
	// a fixed Lua error rather than share a nameless namespace.
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	c := NewCache()
	r := NewWriter(ws.services("/tmp"), &buf, WithCache(c))
	defer r.Close()
	// scriptPath intentionally not set.
	err := mustRunErr(t, r, `rela.cache.get("k")`)
	if !strings.Contains(err.Error(), "not available in inline/eval") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetAndGetRoundTrip(t *testing.T) {
	r, _ := newCachedWriter(t, "script-a.lua")
	mustRun(t, r, `
		rela.cache.set("k", 42)
		local v = rela.cache.get("k")
		assert(v == 42, "expected 42 got " .. tostring(v))
	`)
}

func TestCacheSetNilDeletes(t *testing.T) {
	// Passing nil as the value is the only delete surface exposed to
	// Lua; it matches Lua table semantics (nil = absent).
	r, _ := newCachedWriter(t, "s.lua")
	mustRun(t, r, `
		rela.cache.set("k", "hello")
		assert(rela.cache.get("k") == "hello", "expected hello after set")
		rela.cache.set("k", nil)
		assert(rela.cache.get("k") == nil, "expected nil after set(nil)")
	`)
}

func TestCacheGetRejectsOptions(t *testing.T) {
	// get takes no options; passing any table should fail loudly so a
	// user who expected opts to do something sees the mistake.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `rela.cache.get("k", {ttl = 60})`)
	if !strings.Contains(err.Error(), "takes 1 argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetRejectsFunction(t *testing.T) {
	// Functions/userdata/coroutines hold live state; caching them is
	// always a mistake. Reject at set-time with a message naming the
	// offending type.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `rela.cache.set("k", function() end)`)
	if !strings.Contains(err.Error(), "cannot cache value of type function") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetRejectsNestedFunction(t *testing.T) {
	// The representability walk recurses into tables; a function
	// buried inside a map still surfaces.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `rela.cache.set("k", {data = {fn = function() end}})`)
	if !strings.Contains(err.Error(), "cannot cache value of type function") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetRejectsCyclicTable(t *testing.T) {
	// Without cycle detection, the walk eats the Go stack until the
	// runtime crashes the whole process with "stack exceeds 1000000000
	// byte limit" — not catchable from PCall. A Lua error at the cache
	// boundary is the only safe way out.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `
		local t = {}
		t.self = t
		rela.cache.set("k", t)
	`)
	if !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetRejectsIndirectCycle(t *testing.T) {
	// Cycle through two tables (a.child = b, b.parent = a) is the
	// realistic case — direct self-reference is rarer than a pair
	// of entities that link to each other.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `
		local a = {}
		local b = {parent = a}
		a.child = b
		rela.cache.set("k", a)
	`)
	if !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogFieldsShortKeyEmitsUnnamespacedMarker(t *testing.T) {
	// Keys constructed via requireCacheContext always have a 16-char
	// hex prefix (the namespace hash). A key shorter than 16 chars
	// could only arrive if a Go caller bypassed requireCacheContext
	// and called cache.get/set directly. When that happens, log
	// operators should see a literal sentinel instead of a truncated
	// slice that looks like a plausible hash.
	fields := logFields("miss", "short")
	for i := 0; i < len(fields); i += 2 {
		k, _ := fields[i].(string)
		if k != "namespace_hash" {
			continue
		}
		v, _ := fields[i+1].(string)
		if v != unnamespacedMarker {
			t.Errorf("expected %q, got %q", unnamespacedMarker, v)
		}
		return
	}
	t.Fatal("namespace_hash field missing from log fields")
}

func TestLuaValueToGoHandlesCycleWithoutCrash(t *testing.T) {
	// Regression: before cycle detection in luaValueToGo, rela.output on
	// a self-referential table crashed the whole process with a Go
	// stack overflow (not a Lua error, not a panic — a fatal runtime
	// termination). Verify it now returns cleanly. Exercises the fix
	// outside the cache boundary where the earlier validateRepresentable
	// fix didn't apply.
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()

	err := r.RunString(`
		local t = {}
		t.self = t
		rela.output(t)
	`)
	if err != nil {
		t.Fatalf("RunString errored unexpectedly: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cyclic reference") {
		t.Errorf("expected output to contain cycle marker, got: %s", out)
	}
}

func TestCacheSetAcceptsSharedNoncyclicTable(t *testing.T) {
	// Two branches reaching the same (non-cyclic) inner table should
	// not be false-positive-rejected. The cycle detector must unwind
	// its ancestry set between siblings.
	r, _ := newCachedWriter(t, "s.lua")
	mustRun(t, r, `
		local shared = {x = 1}
		local outer = {left = shared, right = shared}
		rela.cache.set("k", outer)
		local v = rela.cache.get("k")
		assert(v.left.x == 1 and v.right.x == 1, "shared branch lost")
	`)
}

func TestCacheRejectsOversizedTTL(t *testing.T) {
	// A TTL that overflows float64→int64 would be interpreted as a
	// large negative Duration and the entry would be born already
	// expired, masquerading as a silently broken cache.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `rela.cache.set("k", "v", {ttl = 1e20})`)
	if !strings.Contains(err.Error(), "ttl too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetRejectsLongKey(t *testing.T) {
	r, _ := newCachedWriter(t, "s.lua")
	// 513-byte key.
	err := mustRunErr(t, r, `rela.cache.set(string.rep("x", 513), "v")`)
	if !strings.Contains(err.Error(), "key length 513") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheSetAcceptsMaxKey(t *testing.T) {
	// Exactly 512 is the boundary — must succeed.
	r, _ := newCachedWriter(t, "s.lua")
	mustRun(t, r, `
		local k = string.rep("x", 512)
		rela.cache.set(k, "ok")
		assert(rela.cache.get(k) == "ok")
	`)
}

func TestCacheSetRejectsUnknownOption(t *testing.T) {
	// Typos like `refersh` are caught because we allowlist option names.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `rela.cache.set("k", "v", {refersh = true})`)
	if !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheMemoizeHitSkipsFn(t *testing.T) {
	// Second call to memoize must not invoke fn — that's the whole
	// point of caching. Use a shared upvalue counter to detect extra
	// calls.
	r, _ := newCachedWriter(t, "s.lua")
	mustRun(t, r, `
		local calls = 0
		local function compute()
			calls = calls + 1
			return "result"
		end
		local a = rela.cache.memoize("k", compute)
		local b = rela.cache.memoize("k", compute)
		assert(a == "result" and b == "result", "values not equal")
		assert(calls == 1, "expected 1 call, got " .. calls)
	`)
}

func TestCacheMemoizeMultipleReturns(t *testing.T) {
	// Lua fns routinely return (value, err); dropping anything past
	// the first is a silent footgun. Memoize captures and re-emits
	// ALL returns — this is the core AC-11 guarantee.
	r, _ := newCachedWriter(t, "s.lua")
	mustRun(t, r, `
		local function mk() return 1, "two", {three = 3} end
		local a, b, c = rela.cache.memoize("k", mk)
		assert(a == 1, "a")
		assert(b == "two", "b")
		assert(type(c) == "table" and c.three == 3, "c")

		-- On hit, all three must come back again.
		local x, y, z = rela.cache.memoize("k", mk)
		assert(x == 1 and y == "two" and z.three == 3, "hit lost data")
	`)
}

func TestCacheMemoizeFnRaisesNotCached(t *testing.T) {
	// If fn errors, cache stays empty so the next call re-runs it
	// (maybe successfully this time). Store-on-error would pin a
	// useless failure permanently.
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `
		rela.cache.memoize("k", function() error("boom") end)
	`)
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to wrap fn error: %v", err)
	}
	// Next call should still miss — if it hits, we stored a failure.
	mustRun(t, r, `
		local called = false
		local function good()
			called = true
			return "ok"
		end
		local v = rela.cache.memoize("k", good)
		assert(v == "ok")
		assert(called, "fn not called — cache stored previous failure!")
	`)
}

func TestCacheMemoizeBypass(t *testing.T) {
	// bypass=true skips the read but still writes. Useful when the
	// caller knows the cached value is stale.
	r, _ := newCachedWriter(t, "s.lua")
	mustRun(t, r, `
		local calls = 0
		local function f()
			calls = calls + 1
			return calls
		end
		local a = rela.cache.memoize("k", f)
		local b = rela.cache.memoize("k", f, {bypass = true})
		assert(a == 1, "first call")
		assert(b == 2, "bypass re-ran fn, got " .. b)
		-- After bypass the cache holds the latest.
		local c = rela.cache.get("k")
		assert(c == 2, "cache not updated by bypass-memoize")
	`)
}

func TestCacheMemoizeRejectsUnknownOption(t *testing.T) {
	r, _ := newCachedWriter(t, "s.lua")
	err := mustRunErr(t, r, `
		rela.cache.memoize("k", function() return 1 end, {refersh = true})
	`)
	if !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheNamespacedIsolation(t *testing.T) {
	// Two runtimes with different scriptPath must NOT see each other's
	// entries even when using the exact same user key. That's the
	// namespacing contract.
	ws := newMockWorkspace(t)
	c := NewCache()

	a := NewWriter(ws.services("/tmp"), &bytes.Buffer{}, WithCache(c))
	defer a.Close()
	a.SetScriptPath("a.lua")

	b := NewWriter(ws.services("/tmp"), &bytes.Buffer{}, WithCache(c))
	defer b.Close()
	b.SetScriptPath("b.lua")

	mustRun(t, a, `rela.cache.set("shared", "from-a")`)
	mustRun(t, b, `rela.cache.set("shared", "from-b")`)

	mustRun(t, a, `assert(rela.cache.get("shared") == "from-a")`)
	mustRun(t, b, `assert(rela.cache.get("shared") == "from-b")`)
}

func TestCacheTTLExpiry(t *testing.T) {
	// TTL is lazy — the expired entry hangs around until next touch.
	// Use an injected time source so the test doesn't sleep.
	r, c := newCachedWriter(t, "s.lua")

	now := time.Now()
	c.SetNow(func() time.Time { return now })

	mustRun(t, r, `rela.cache.set("k", "v", {ttl = 10})`)
	mustRun(t, r, `assert(rela.cache.get("k") == "v", "pre-expiry")`)

	// Jump past the TTL.
	now = now.Add(11 * time.Second)
	mustRun(t, r, `assert(rela.cache.get("k") == nil, "post-expiry")`)
}

func TestCacheTTLZeroNeverExpires(t *testing.T) {
	// An explicit ttl=0 (or negative) means "store forever". It's a
	// valid opt-in for expensive results the script knows won't go
	// stale.
	r, c := newCachedWriter(t, "s.lua")

	now := time.Now()
	c.SetNow(func() time.Time { return now })

	mustRun(t, r, `rela.cache.set("k", "v", {ttl = 0})`)
	// Jump far into the future.
	now = now.Add(100 * 365 * 24 * time.Hour)
	mustRun(t, r, `assert(rela.cache.get("k") == "v", "zero-ttl expired")`)
}

func TestCacheLRUEvictionAtCap(t *testing.T) {
	// Fill the cache past its cap and assert the least-recently-accessed
	// entry is evicted. Use a monotonic fake clock so every entry has
	// a distinct lastAccess — real time.Now can tie.
	r, c := newCachedWriter(t, "s.lua")

	var clock time.Time
	clock = time.Now()
	c.SetNow(func() time.Time {
		clock = clock.Add(time.Microsecond)
		return clock
	})

	// Write maxCacheEntries entries — "0" is the oldest.
	for i := range maxCacheEntries {
		if err := r.RunString(setCode(i)); err != nil {
			t.Fatalf("set %d failed: %v", i, err)
		}
	}
	// One more triggers eviction.
	if err := r.RunString(setCode(maxCacheEntries)); err != nil {
		t.Fatalf("set overflow failed: %v", err)
	}

	// "0" should be gone, the second oldest ("1") should still be there,
	// and the newest (maxCacheEntries) should be there.
	mustRun(t, r, `
		assert(rela.cache.get("e0") == nil, "oldest not evicted")
		assert(rela.cache.get("e1") == "v1", "second-oldest missing")
	`)
	if err := r.RunString(getCheckCode(maxCacheEntries)); err != nil {
		t.Fatalf("get newest failed: %v", err)
	}
}

func setCode(i int) string {
	return "rela.cache.set('e" + itoa(i) + "', 'v" + itoa(i) + "')"
}

func getCheckCode(i int) string {
	return "assert(rela.cache.get('e" + itoa(i) + "') == 'v" + itoa(i) +
		"', 'missing e" + itoa(i) + "')"
}

// itoa avoids strconv to keep the test tight.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestCacheSetDeletePersistsAcrossGet(t *testing.T) {
	// Tests that set(nil) also wipes the in-memory entry, not just
	// some flagged-as-deleted state. Uses the Cache directly.
	c := NewCache()
	c.set("ns\x00k", []interface{}{"v"}, 0)
	if v, ok := c.get("ns\x00k"); !ok || len(v) != 1 || v[0] != "v" {
		t.Fatalf("unexpected state: %v ok=%v", v, ok)
	}
	c.delete("ns\x00k")
	if _, ok := c.get("ns\x00k"); ok {
		t.Fatal("entry survived delete")
	}
}

func TestCacheMemoizeConcurrentBothRun(t *testing.T) {
	// Two goroutines hit memoize on the same key at the same time.
	// The mutex is released across fn, so both fn instances run; the
	// later write wins. This is strictly correct and avoids holding
	// a lock across arbitrary script code.
	//
	// Use two independent runtimes sharing one Cache so the gopher-lua
	// VM (which is single-threaded per LState) isn't the one we're
	// stressing.
	ws := newMockWorkspace(t)
	c := NewCache()

	r1 := NewWriter(ws.services("/tmp"), &bytes.Buffer{}, WithCache(c))
	defer r1.Close()
	r1.SetScriptPath("s.lua")
	r2 := NewWriter(ws.services("/tmp"), &bytes.Buffer{}, WithCache(c))
	defer r2.Close()
	r2.SetScriptPath("s.lua") // Same namespace = same cache entry.

	var wg sync.WaitGroup
	var mu sync.Mutex
	var calls int

	// A Go-callable to record invocations. Injected via global.
	count := func() {
		mu.Lock()
		calls++
		mu.Unlock()
	}
	for _, r := range []*Runtime{r1, r2} {
		r.LState().SetGlobal("count", r.LState().NewFunction(func(_ *glua.LState) int {
			count()
			return 0
		}))
	}

	code := `
		rela.cache.memoize("shared", function()
			count()
			return "val"
		end)
	`

	// Start both; use a start-gate so they race.
	startCh := make(chan struct{})
	for _, r := range []*Runtime{r1, r2} {
		wg.Add(1)
		go func(rt *Runtime) {
			defer wg.Done()
			<-startCh
			if err := rt.RunString(code); err != nil {
				t.Errorf("run failed: %v", err)
			}
		}(r)
	}
	close(startCh)
	wg.Wait()

	// In the worst (fastest) case the two goroutines serialize and the
	// second sees a cache hit → calls=1. In the common race case both
	// miss and both call fn → calls=2. Anything outside [1, 2] is a
	// bug.
	mu.Lock()
	defer mu.Unlock()
	if calls < 1 || calls > 2 {
		t.Fatalf("unexpected call count %d (want 1 or 2)", calls)
	}
}

func TestCacheLoggingNeverLeaksRawKey(t *testing.T) {
	// Capture slog output and assert neither the raw key nor the raw
	// script path ever appear. This is a defense-in-depth check in
	// addition to the "never log raw" code path.
	var logBuf bytes.Buffer
	prev := slog.Default()
	h := slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(prev)

	const (
		secretKey  = "SECRET-KEY-MARKER"
		secretPath = "SECRET-PATH-MARKER.lua"
	)
	r, _ := newCachedWriter(t, secretPath)

	mustRun(t, r, `rela.cache.set("`+secretKey+`", "v")`)
	mustRun(t, r, `rela.cache.get("`+secretKey+`")`)

	out := logBuf.String()
	if strings.Contains(out, secretKey) {
		t.Errorf("logs leaked raw key: %s", out)
	}
	if strings.Contains(out, secretPath) {
		t.Errorf("logs leaked raw path: %s", out)
	}
	// And verify we DID log hashed fields (sanity check that the
	// leak-free-ness isn't just "we didn't log anything").
	if !strings.Contains(out, "key_hash") || !strings.Contains(out, "namespace_hash") {
		t.Errorf("expected hash fields in log output:\n%s", out)
	}
}

func TestCacheErrorMessagesDoNotLeakKey(t *testing.T) {
	// Error paths that include user-controlled values must never format
	// the raw key into the message. Check each rejection path by
	// injecting a marker and asserting the marker is absent from the
	// returned error.
	const marker = "SECRET-SUBSTRING-9999"
	r, _ := newCachedWriter(t, "s.lua")

	// 1. Unrepresentable value — marker is the key.
	err := mustRunErr(t, r, `rela.cache.set("`+marker+`-key", function() end)`)
	if strings.Contains(err.Error(), marker) {
		t.Errorf("unrepresentable-value error leaked raw key: %s", err)
	}

	// 2. Long key — the marker lives inside the key, the error reports
	// the length only.
	longKey := marker + strings.Repeat("x", 600)
	err = mustRunErr(t, r, `rela.cache.set("`+longKey+`", "v")`)
	if strings.Contains(err.Error(), marker) {
		t.Errorf("long-key error leaked raw key: %s", err)
	}
	if !strings.Contains(err.Error(), "key length") {
		t.Errorf("long-key error doesn't report length: %s", err)
	}

	// 3. Unknown option — the marker is the option name; error lists
	// recognized options but not the rejected one's raw string value
	// (the option name itself may be "refersh" or similar; we care
	// about not formatting a potentially secret-bearing payload value).
	err = mustRunErr(t, r, `rela.cache.set("k", "v", {bogus="`+marker+`"})`)
	if strings.Contains(err.Error(), marker) {
		t.Errorf("unknown-option error leaked raw value: %s", err)
	}
}

func TestCacheBehaviourWithNilCacheOption(t *testing.T) {
	// WithCache(nil) is equivalent to omitting it — rela.cache.* stays
	// unregistered. This lets callers that deliberately want no caching
	// pass a nil explicitly.
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf, WithCache(nil))
	defer r.Close()
	r.SetScriptPath("foo.lua")
	err := r.RunString(`rela.cache.get("k")`)
	if err == nil {
		t.Fatal("expected error — nil cache should not register bindings")
	}
}

func TestCacheRunFileSetsScriptPath(t *testing.T) {
	// Sanity check: RunFile wires scriptPath automatically. Without
	// this, file-loaded scripts would hit the inline/eval guard.
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf, WithCache(NewCache()))
	defer r.Close()

	// Use a tmp file with actual Lua code.
	code := `rela.cache.set("k", "v"); assert(rela.cache.get("k") == "v")`
	tmp := t.TempDir() + "/test.lua"
	if err := writeFileString(tmp, code); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := r.RunFile(tmp, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}
	if r.scriptPath == "" {
		t.Fatal("RunFile did not set scriptPath")
	}
}

// writeFileString is an in-package test helper.
func writeFileString(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
