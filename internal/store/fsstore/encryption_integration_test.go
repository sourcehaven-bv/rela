package fsstore_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// These tests exercise the full read/write path with an encryption
// policy wired in, validating that entity.Properties values round-trip
// correctly through the on-disk wire format. They use a hand-rolled
// Crypto adapter rather than the production one (which lives in
// internal/app) to avoid a cyclical test dependency.

// --- test crypto adapter --------------------------------------------

type testCrypto struct {
	propGroups map[string]map[string]string // type → prop → group
	bodyGroups map[string]string            // type → group
	groups     map[string][]string          // group → []identity
	pubKeys    map[string]*encryption.PublicKey
	private    *encryption.Keypair // nil = no local private key
	me         string              // identity name of `private`
}

func (c *testCrypto) PropertyGroup(t, p string) (string, bool) {
	if g, ok := c.propGroups[t][p]; ok {
		return g, true
	}
	return "", false
}

func (c *testCrypto) BodyGroup(t string) (string, bool) {
	if g, ok := c.bodyGroups[t]; ok && g != "" {
		return g, true
	}
	return "", false
}

func (c *testCrypto) Recipients(g string) ([]string, bool) {
	ids, ok := c.groups[g]
	return ids, ok
}

func (c *testCrypto) Recipient(id string) (*encryption.PublicKey, bool) {
	p, ok := c.pubKeys[id]
	return p, ok
}

func (c *testCrypto) HasPrivateKey() bool { return c.private != nil }

func (c *testCrypto) UnwrapAny(wraps map[string][]byte) (dataKey []byte, matched string, err error) {
	if c.private == nil {
		return nil, "", encryption.ErrNoPrivateKey
	}
	if w, ok := wraps[c.me]; ok {
		dk, err := encryption.UnwrapKey(w, c.private)
		if err != nil {
			return nil, "", err
		}
		return dk, c.me, nil
	}
	return nil, "", encryption.ErrNoMatchingKey
}

// encFixture holds two keypairs and a shared pubkey map so tests can
// easily swap between "I am alice" and "I am eve" perspectives.
type encFixture struct {
	alice *encryption.Keypair
	bob   *encryption.Keypair
	eve   *encryption.Keypair
	pub   map[string]*encryption.PublicKey
}

func newEncFixture(t *testing.T) *encFixture {
	t.Helper()
	a, err := encryption.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	b, err := encryption.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	e, err := encryption.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return &encFixture{
		alice: a, bob: b, eve: e,
		pub: map[string]*encryption.PublicKey{
			"alice": a.PublicKey(),
			"bob":   b.PublicKey(),
			"eve":   e.PublicKey(),
		},
	}
}

// ticketCrypto builds a Crypto adapter where the ticket type has
// description encrypted for engineering:[alice,bob].
func (f *encFixture) ticketCrypto(as string) *testCrypto {
	var priv *encryption.Keypair
	switch as {
	case "alice":
		priv = f.alice
	case "bob":
		priv = f.bob
	case "eve":
		priv = f.eve
	}
	return &testCrypto{
		propGroups: map[string]map[string]string{
			"ticket": {"description": "engineering"},
		},
		bodyGroups: map[string]string{},
		groups: map[string][]string{
			"engineering": {"alice", "bob"},
		},
		pubKeys: f.pub,
		private: priv,
		me:      as,
	}
}

func newEncryptedStore(t *testing.T, fs storage.FS, crypto fsstore.Crypto) *fsstore.FSStore {
	t.Helper()
	cfg := fsstore.Config{
		FS:             fs,
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
		Schemas: map[string]store.EntityTypeSchema{
			"ticket": {
				Plural:        "tickets",
				PropertyOrder: []string{"title", "description", "status"},
			},
		},
		Crypto: crypto,
	}
	s, err := fsstore.New(cfg)
	if err != nil {
		t.Fatalf("fsstore.New: %v", err)
	}
	return s
}

// --- tests -----------------------------------------------------------

func TestEncryption_RoundTripThroughFSStore(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)
	s := newEncryptedStore(t, fs, f.ticketCrypto("alice"))

	ctx := context.Background()
	original := entity.New("TKT-001", "ticket")
	original.Properties["title"] = "Fix auth"
	original.Properties["description"] = "TOP SECRET — reset the HSM"
	original.Properties["status"] = "open"

	if err := s.CreateEntity(ctx, original); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	// Read back through the store.
	got, err := s.GetEntity(ctx, "TKT-001")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Properties["description"] != "TOP SECRET — reset the HSM" {
		t.Errorf("description = %v, want original", got.Properties["description"])
	}
	if got.Properties["title"] != "Fix auth" {
		t.Errorf("title lost: %v", got.Properties["title"])
	}

	// Spot-check the on-disk file: description is under _enc_v1_
	// key, not under its plain name; cleartext fields remain plain.
	path := "/entities/tickets/TKT-001.md"
	raw, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "TOP SECRET") {
		t.Errorf("plaintext leaked to disk:\n%s", raw)
	}
	if !strings.Contains(string(raw), "_enc_v1_description") {
		t.Errorf("on-disk file missing _enc_v1_description key:\n%s", raw)
	}
	if !strings.Contains(string(raw), "_encryption") {
		t.Errorf("on-disk file missing _encryption block:\n%s", raw)
	}
	if !strings.Contains(string(raw), "title: Fix auth") {
		t.Errorf("cleartext title not present on disk:\n%s", raw)
	}
}

func TestEncryption_WrongKeyGivesOpaque(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)

	// Alice writes.
	sAlice := newEncryptedStore(t, fs, f.ticketCrypto("alice"))
	ctx := context.Background()
	e := entity.New("TKT-002", "ticket")
	e.Properties["title"] = "Alice's ticket"
	e.Properties["description"] = "confidential"
	if err := sAlice.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	// Eve reads. She has a keyring but isn't a recipient.
	sEve := newEncryptedStore(t, fs, f.ticketCrypto("eve"))
	got, err := sEve.GetEntity(ctx, "TKT-002")
	if err != nil {
		t.Fatalf("eve GetEntity: %v", err)
	}
	// Cleartext visible.
	if got.Properties["title"] != "Alice's ticket" {
		t.Errorf("title lost: %v", got.Properties["title"])
	}
	// Encrypted value surfaced as Opaque.
	op, ok := got.Properties["description"].(encryption.Opaque)
	if !ok {
		t.Fatalf("description = %T, want encryption.Opaque", got.Properties["description"])
	}
	if op.String() != "<encrypted>" {
		t.Errorf("Opaque.String() = %q, want <encrypted>", op.String())
	}
}

func TestEncryption_WrongKey_WritePartialRefused(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)

	// Alice writes.
	sAlice := newEncryptedStore(t, fs, f.ticketCrypto("alice"))
	ctx := context.Background()
	e := entity.New("TKT-003", "ticket")
	e.Properties["title"] = "T"
	e.Properties["description"] = "S"
	if err := sAlice.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	// Eve reads + attempts update.
	sEve := newEncryptedStore(t, fs, f.ticketCrypto("eve"))
	got, err := sEve.GetEntity(ctx, "TKT-003")
	if err != nil {
		t.Fatal(err)
	}
	got.Properties["title"] = "edited by eve"
	err = sEve.UpdateEntity(ctx, got)
	if err == nil {
		t.Fatal("expected OpaqueWrite refusal")
	}
	var ee *fsstore.EncryptionError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v, want fsstore.EncryptionError", err)
	}
	if ee.Kind != fsstore.ErrKindOpaqueWrite {
		t.Errorf("Kind = %s, want OpaqueWrite", ee.Kind)
	}
}

func TestEncryption_MultiRecipient_BobReads(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)

	sAlice := newEncryptedStore(t, fs, f.ticketCrypto("alice"))
	ctx := context.Background()
	e := entity.New("TKT-004", "ticket")
	e.Properties["title"] = "t"
	e.Properties["description"] = "alice and bob can read this"
	if err := sAlice.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	sBob := newEncryptedStore(t, fs, f.ticketCrypto("bob"))
	got, err := sBob.GetEntity(ctx, "TKT-004")
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties["description"] != "alice and bob can read this" {
		t.Errorf("bob decrypt = %v", got.Properties["description"])
	}
}

func TestEncryption_BackwardCompat_CleartextStore(t *testing.T) {
	// With Crypto=nil, fsstore must behave exactly as it did before
	// this slice — no _enc_v1_ keys, no _encryption block.
	fs := storage.NewMemFS()
	s := newEncryptedStore(t, fs, nil) // nil Crypto

	ctx := context.Background()
	e := entity.New("TKT-005", "ticket")
	e.Properties["title"] = "plain"
	e.Properties["description"] = "plain too"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	raw, err := fs.ReadFile("/entities/tickets/TKT-005.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "_enc_v1_") {
		t.Errorf("nil Crypto must not emit encrypted keys:\n%s", raw)
	}
	if strings.Contains(string(raw), "_encryption") {
		t.Errorf("nil Crypto must not emit _encryption block:\n%s", raw)
	}
	if !strings.Contains(string(raw), "description: plain too") {
		t.Errorf("cleartext not preserved:\n%s", raw)
	}

	// Round-trip read.
	got, err := s.GetEntity(ctx, "TKT-005")
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties["description"] != "plain too" {
		t.Errorf("round-trip: %v", got.Properties["description"])
	}
}

func TestEncryption_FreshDataKeyPerWrite(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)
	s := newEncryptedStore(t, fs, f.ticketCrypto("alice"))

	ctx := context.Background()
	e := entity.New("TKT-006", "ticket")
	e.Properties["title"] = "t"
	e.Properties["description"] = "same value both writes"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	raw1, _ := fs.ReadFile("/entities/tickets/TKT-006.md")

	// Update with identical contents.
	got, err := s.GetEntity(ctx, "TKT-006")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateEntity(ctx, got); err != nil {
		t.Fatal(err)
	}
	raw2, _ := fs.ReadFile("/entities/tickets/TKT-006.md")

	if bytes.Equal(raw1, raw2) {
		t.Error("two writes of identical data produced identical disk bytes — data-key reuse?")
	}
}

// encBodyCrypto variant: same as ticketCrypto but the body is
// declared encrypted instead of any property.
func (f *encFixture) encBodyCrypto(as string) *testCrypto {
	c := f.ticketCrypto(as)
	c.propGroups = map[string]map[string]string{} // no encrypted props
	c.bodyGroups = map[string]string{"ticket": "engineering"}
	return c
}

func TestEncryption_BodyRoundTrip(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)
	s := newEncryptedStore(t, fs, f.encBodyCrypto("alice"))

	ctx := context.Background()
	e := entity.New("TKT-007", "ticket")
	e.Properties["title"] = "t"
	e.Content = "the body is secret too\nline 2\n"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetEntity(ctx, "TKT-007")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "the body is secret too\nline 2\n" {
		t.Errorf("body = %q", got.Content)
	}

	// Spot-check on-disk file.
	raw, _ := fs.ReadFile("/entities/tickets/TKT-007.md")
	if strings.Contains(string(raw), "the body is secret") {
		t.Errorf("plaintext body leaked to disk:\n%s", raw)
	}
	if !strings.Contains(string(raw), "_encrypted_body") {
		t.Errorf("_encrypted_body key missing:\n%s", raw)
	}
}

func TestEncryption_TamperedFile(t *testing.T) {
	fs := storage.NewMemFS()
	f := newEncFixture(t)
	s := newEncryptedStore(t, fs, f.ticketCrypto("alice"))

	ctx := context.Background()
	e := entity.New("TKT-008", "ticket")
	e.Properties["title"] = "t"
	e.Properties["description"] = "untampered"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	path := "/entities/tickets/TKT-008.md"
	raw, _ := fs.ReadFile(path)
	// Find the _enc_v1_description line and flip a character in the
	// middle of its base64 payload (avoiding the "_enc_v1_description: "
	// key prefix and any trailing padding/whitespace).
	lines := strings.Split(string(raw), "\n")
	tampered := false
	for i, line := range lines {
		const prefix = "_enc_v1_description: "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		payload := strings.TrimRight(line[len(prefix):], " =") // strip padding
		if len(payload) < 8 {
			continue
		}
		mid := len(payload) / 2
		runes := []rune(payload)
		// Toggle a char in the middle. Base64 alphabet: flip 'A'↔'B',
		// otherwise flip to 'A'.
		if runes[mid] == 'A' {
			runes[mid] = 'B'
		} else {
			runes[mid] = 'A'
		}
		mutatedPayload := string(runes) + line[len(prefix)+len(payload):]
		lines[i] = prefix + mutatedPayload
		tampered = true
		break
	}
	if !tampered {
		t.Fatalf("could not find _enc_v1_description line in:\n%s", raw)
	}
	// Write the tampered content back.
	_ = fs.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)

	got, err := s.GetEntity(ctx, "TKT-008")
	// A tampered ciphertext body has two possible outcomes:
	// 1. The decrypt fails and we surface a CorruptedFile error.
	// 2. The decrypt doesn't match any known group key, surfacing
	//    the property as an Opaque.
	// Both are correct — "never recover the original plaintext" is
	// what matters.
	if err != nil {
		var ee *fsstore.EncryptionError
		if errors.As(err, &ee) && ee.Kind == fsstore.ErrKindCorruptedFile {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
	// No error: the property must NOT be the original plaintext.
	desc := got.Properties["description"]
	if s, ok := desc.(string); ok && s == "untampered" {
		t.Fatalf("tamper undetected — recovered plaintext: %v", desc)
	}
	// Also acceptable: Opaque surfacing.
	if _, ok := desc.(encryption.Opaque); !ok {
		t.Fatalf("description = %T (%v), want Opaque or error", desc, desc)
	}
}
