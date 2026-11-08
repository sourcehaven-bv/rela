package peerkey

import (
	"bytes"
	"crypto/rand"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newKey(t *testing.T) (key, digest string) {
	t.Helper()
	key, digest, err := Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key, digest
}

func TestGenerate_Format(t *testing.T) {
	t.Parallel()
	key, digest, err := Generate(bytes.NewReader(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if want := Prefix + strings.Repeat("A", 43); key != want {
		t.Errorf("key = %q, want %q", key, want)
	}
	if digest != Digest(key) || !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
		t.Errorf("digest = %q", digest)
	}
	if _, _, err := Generate(bytes.NewReader(make([]byte, 5))); err == nil {
		t.Error("a short random source was accepted")
	}
	if _, _, err := Generate(nil); err == nil {
		t.Error("a nil random source was accepted")
	}
}

func TestKeyring_Verify(t *testing.T) {
	t.Parallel()
	acme, acmeDigest := newKey(t)
	other, _ := newKey(t)
	ring, err := NewKeyring(map[string]string{"acme": acmeDigest})
	if err != nil {
		t.Fatal(err)
	}
	if ring.Len() != 1 {
		t.Errorf("Len = %d, want 1", ring.Len())
	}
	if name, err := ring.Verify(acme); err != nil || name != "acme" {
		t.Errorf("Verify(acme) = %q, %v", name, err)
	}
	for _, tc := range []struct {
		name string
		key  string
		want error
	}{
		{"unknown key", other, ErrUnknown},
		{"no prefix", strings.TrimPrefix(acme, Prefix), ErrMalformed},
		{"empty", "", ErrMalformed},
		{"prefix only", Prefix, ErrMalformed},
		{"short", acme[:len(acme)-1], ErrMalformed},
		{"long", acme + "A", ErrMalformed},
		{"bad alphabet", acme[:len(acme)-1] + "+", ErrMalformed},
		{"padding", acme[:len(acme)-1] + "=", ErrMalformed},
		{"upper-case prefix", strings.ToUpper(Prefix) + strings.TrimPrefix(acme, Prefix), ErrMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ring.Verify(tc.key); !errors.Is(err, tc.want) {
				t.Errorf("Verify = %v, want %v", err, tc.want)
			}
		})
	}
	var nilRing *Keyring
	if _, err := nilRing.Verify(acme); !errors.Is(err, ErrUnknown) {
		t.Errorf("nil keyring: %v", err)
	}
}

func TestNewKeyring_Refusals(t *testing.T) {
	t.Parallel()
	_, d := newKey(t)
	for _, tc := range []struct {
		name    string
		entries map[string]string
		want    string
	}{
		{"bad name", map[string]string{"Acme": d}, "peer_key.Acme"},
		{"no scheme", map[string]string{"acme": strings.TrimPrefix(d, "sha256:")}, "must start"},
		{"short hex", map[string]string{"acme": d[:len(d)-1]}, "hex digits"},
		{"upper hex", map[string]string{"acme": "sha256:" + strings.ToUpper(strings.TrimPrefix(d, "sha256:"))},
			"hex digits"},
		{"not hex", map[string]string{"acme": "sha256:" + strings.Repeat("z", 64)}, "hex digits"},
		{"duplicate digest", map[string]string{"acme": d, "beta": d}, "same digest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewKeyring(tc.entries)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			for _, v := range tc.entries {
				if strings.Contains(err.Error(), strings.TrimPrefix(v, "sha256:")) && len(v) > 10 {
					t.Errorf("the error echoes a value: %v", err)
				}
			}
		})
	}
}

// fakeClock is a settable clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestSource_ReloadsAfterInterval(t *testing.T) {
	t.Parallel()
	key, digest := newKey(t)
	var mu sync.Mutex
	entries := map[string]string{"acme": digest}
	loads := 0
	load := func() (map[string]string, error) {
		mu.Lock()
		defer mu.Unlock()
		loads++
		return entries, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	s, err := NewSource(load, ReloadInterval, clock.now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if name, err := s.Verify(key); err != nil || name != "acme" {
		t.Fatalf("Verify = %q, %v", name, err)
	}
	// Revoke; the old keyring serves until the interval passes.
	mu.Lock()
	entries = map[string]string{}
	mu.Unlock()
	clock.advance(ReloadInterval - time.Second)
	if _, err := s.Verify(key); err != nil {
		t.Fatalf("revoked too early: %v", err)
	}
	clock.advance(time.Second)
	if _, err := s.Verify(key); !errors.Is(err, ErrUnknown) {
		t.Fatalf("revoked key still verifies: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if loads != 2 {
		t.Errorf("loads = %d, want 2", loads)
	}
}

func TestSource_BadListFailsClosed(t *testing.T) {
	t.Parallel()
	key, digest := newKey(t)
	var mu sync.Mutex
	fail := false
	load := func() (map[string]string, error) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return map[string]string{"acme": digest, "bad name": "x"}, nil
		}
		return map[string]string{"acme": digest}, nil
	}
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))
	clock := &fakeClock{t: time.Unix(1000, 0)}
	s, err := NewSource(load, time.Second, clock.now, log)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Verify(key); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	fail = true
	mu.Unlock()
	clock.advance(time.Second)
	if _, err = s.Verify(key); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a broken list kept a key working: %v", err)
	}
	if !strings.Contains(logBuf.String(), "peer_key.bad name") || strings.Contains(logBuf.String(), digest) {
		t.Errorf("log = %q", logBuf.String())
	}

	loadErr := func() (map[string]string, error) { return nil, errors.New("boom") }
	s2, err := NewSource(loadErr, time.Second, clock.now, log)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Verify(key); !errors.Is(err, ErrUnknown) {
		t.Fatalf("an unreadable list verified a key: %v", err)
	}
}

func TestSource_ConcurrentVerify(t *testing.T) {
	t.Parallel()
	key, digest := newKey(t)
	load := func() (map[string]string, error) { return map[string]string{"acme": digest}, nil }
	clock := &fakeClock{t: time.Unix(1000, 0)}
	s, err := NewSource(load, time.Millisecond, clock.now, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 50 {
				clock.advance(time.Millisecond)
				// The clock races ahead of a reload in progress, so a
				// stale refusal is a correct answer here; nothing else is.
				if _, err := s.Verify(key); err != nil && !errors.Is(err, ErrStale) {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestSource_HungReloadFailsClosed: while one request's reload hangs, the
// others keep the last keyring for a while, then refuse every key.
func TestSource_HungReloadFailsClosed(t *testing.T) {
	t.Parallel()
	key, digest := newKey(t)
	release := make(chan struct{})
	started := make(chan struct{})
	var calls atomic.Int32
	load := func() (map[string]string, error) {
		if calls.Add(1) == 2 {
			close(started)
			<-release
		}
		return map[string]string{"acme": digest}, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	s, err := NewSource(load, time.Second, clock.now, nil)
	if err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.Verify(key) // takes the reload and hangs in load
	}()
	<-started
	if _, err = s.Verify(key); err != nil {
		t.Fatalf("within the bound: %v", err)
	}
	clock.advance(staleFactor * time.Second)
	if _, err = s.Verify(key); !errors.Is(err, ErrStale) {
		t.Fatalf("past the bound: %v, want ErrStale", err)
	}
	close(release)
	<-done
}

func TestNewSource_Refusals(t *testing.T) {
	t.Parallel()
	load := func() (map[string]string, error) { return map[string]string{}, nil }
	if _, err := NewSource(nil, time.Second, time.Now, nil); err == nil {
		t.Error("nil loader accepted")
	}
	if _, err := NewSource(load, time.Second, nil, nil); err == nil {
		t.Error("nil clock accepted")
	}
	if _, err := NewSource(load, 0, time.Now, nil); err == nil {
		t.Error("zero interval accepted")
	}
}

func TestSecretsLoader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, err := SecretsLoader(dir)()
	if err != nil || len(got) != 0 {
		t.Fatalf("no file: %v, %v", got, err)
	}
	_, digest := newKey(t)
	content := "api_key: x\npeer_key.acme: " + digest + "\n"
	if err = os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = SecretsLoader(dir)()
	if err != nil || got["acme"] != digest || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(":\n\t- ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SecretsLoader(dir)(); err == nil {
		t.Error("a malformed file was accepted")
	}
}
