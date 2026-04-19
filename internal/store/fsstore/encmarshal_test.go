package fsstore

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// --- fake Crypto for unit tests ----------------------------------------

// fakeCrypto is a minimal test double that wraps a real Keyring + a
// policy table, so we don't have to stand up the full metamodel or
// production adapter just to exercise seal/unseal.
type fakeCrypto struct {
	propGroups map[string]map[string]string // type → prop → group
	bodyGroups map[string]string            // type → group ("" = none)
	groups     map[string][]string          // group → identity list
	pubKeys    map[string]*encryption.PublicKey
	privateKey *encryption.Keypair // nil = no local private key
	identity   string              // identity name of privateKey
}

func (c *fakeCrypto) PropertyGroup(entityType, property string) (string, bool) {
	if g, ok := c.propGroups[entityType][property]; ok {
		return g, true
	}
	return "", false
}

func (c *fakeCrypto) BodyGroup(entityType string) (string, bool) {
	g := c.bodyGroups[entityType]
	if g == "" {
		return "", false
	}
	return g, true
}

func (c *fakeCrypto) Recipients(group string) ([]string, bool) {
	ids, ok := c.groups[group]
	return ids, ok
}

func (c *fakeCrypto) Recipient(identity string) (*encryption.PublicKey, bool) {
	p, ok := c.pubKeys[identity]
	return p, ok
}

func (c *fakeCrypto) HasPrivateKey() bool { return c.privateKey != nil }

func (c *fakeCrypto) UnwrapAny(wraps map[string][]byte) (dataKey []byte, matched string, err error) {
	if c.privateKey == nil {
		return nil, "", encryption.ErrNoPrivateKey
	}
	// If our identity is among the offered wraps, try to unwrap it.
	if w, ok := wraps[c.identity]; ok {
		dk, err := encryption.UnwrapKey(w, c.privateKey)
		if err != nil {
			return nil, "", err
		}
		return dk, c.identity, nil
	}
	return nil, "", encryption.ErrNoMatchingKey
}

// --- fixtures ----------------------------------------------------------

type fixture struct {
	alice *encryption.Keypair
	bob   *encryption.Keypair
	eve   *encryption.Keypair
	// pub maps identity → public key for the Crypto fake.
	pub map[string]*encryption.PublicKey
}

func newFixture(t *testing.T) *fixture {
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
	return &fixture{
		alice: a,
		bob:   b,
		eve:   e,
		pub: map[string]*encryption.PublicKey{
			"alice": a.PublicKey(),
			"bob":   b.PublicKey(),
			"eve":   e.PublicKey(),
		},
	}
}

// cryptoAs builds a fake Crypto that treats `identity` as the local
// private key and uses engineering:[alice,bob] / exec:[bob,charlie]
// style groups by default.
func (f *fixture) cryptoAs(identity string) *fakeCrypto {
	var priv *encryption.Keypair
	switch identity {
	case "alice":
		priv = f.alice
	case "bob":
		priv = f.bob
	case "eve":
		priv = f.eve
	case "":
		priv = nil
	default:
		panic("unknown identity: " + identity)
	}
	return &fakeCrypto{
		propGroups: map[string]map[string]string{
			"ticket": {"description": "engineering", "secret": "exec"},
		},
		bodyGroups: map[string]string{},
		groups: map[string][]string{
			"engineering": {"alice", "bob"},
			"exec":        {"bob"},
		},
		pubKeys:    f.pub,
		privateKey: priv,
		identity:   identity,
	}
}

// --- tests: key-prefix helpers ----------------------------------------

func TestStripApplyEncKey(t *testing.T) {
	if out, ok := stripEncKey("_enc_v1_description"); !ok || out != "description" {
		t.Errorf("stripEncKey(_enc_v1_description) = (%q, %v)", out, ok)
	}
	if _, ok := stripEncKey("description"); ok {
		t.Error("stripEncKey on plain name should report ok=false")
	}
	if got := applyEncKey("description"); got != "_enc_v1_description" {
		t.Errorf("applyEncKey(description) = %q", got)
	}
	// Round trip.
	orig := "some-prop"
	if back, ok := stripEncKey(applyEncKey(orig)); !ok || back != orig {
		t.Errorf("round trip broken: back=%q ok=%v", back, ok)
	}
}

// --- tests: nil-crypto fast path --------------------------------------

func TestSealProperties_NilCryptoPassThrough(t *testing.T) {
	props := map[string]any{"title": "Hello", "description": "world"}
	order := []string{"title", "description"}
	got, gotOrder, gotBody, err := sealProperties(nil, "ticket", props, "body text", order)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, props) {
		t.Errorf("props changed: %v", got)
	}
	if !reflect.DeepEqual(gotOrder, order) {
		t.Errorf("order changed: %v", gotOrder)
	}
	if gotBody != "body text" {
		t.Errorf("body changed: %q", gotBody)
	}
}

func TestUnsealProperties_NilCrypto_Cleartext(t *testing.T) {
	// No _enc_v1_* keys in frontmatter — must pass through unchanged.
	fm := map[string]any{"title": "Hello", "status": "open"}
	out, body, opaque, err := unsealProperties(nil, fm, "body")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, fm) {
		t.Errorf("fm changed: %v", out)
	}
	if body != "body" {
		t.Errorf("body changed: %q", body)
	}
	if len(opaque) != 0 {
		t.Errorf("expected no opaque props, got %v", opaque)
	}
}

func TestUnsealProperties_NilCrypto_OnEncrypted_Errors(t *testing.T) {
	fm := map[string]any{
		"title":               "Hello",
		"_enc_v1_description": "ignored",
		"_encryption":         map[string]any{},
	}
	_, _, _, err := unsealProperties(nil, fm, "") //nolint:dogsled // test asserts error-only contract
	var ee *EncryptionError
	if !errors.As(err, &ee) || ee.Kind != ErrKindMissingKeyring {
		t.Errorf("err = %v, want MissingKeyring", err)
	}
}

// --- tests: property round-trip ---------------------------------------

func TestSealUnseal_RoundTrip_SingleProperty(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")

	props := map[string]any{"title": "Hello", "description": "top secret"}
	order := []string{"title", "description"}

	sealed, sealedOrder, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}

	// Must contain _enc_v1_description, NOT plain description.
	if _, ok := sealed["_enc_v1_description"]; !ok {
		t.Fatal("sealed map missing _enc_v1_description")
	}
	if _, ok := sealed["description"]; ok {
		t.Fatal("sealed map still has plain description")
	}
	// _encryption block present.
	if _, ok := sealed["_encryption"]; !ok {
		t.Fatal("sealed map missing _encryption")
	}
	// title stays cleartext.
	if sealed["title"] != "Hello" {
		t.Errorf("title = %v, want Hello", sealed["title"])
	}
	// Order: title slot stays, description slot renamed in place.
	wantOrder := []string{"title", "_enc_v1_description", "_encryption"}
	if !reflect.DeepEqual(sealedOrder, wantOrder) {
		t.Errorf("order = %v, want %v", sealedOrder, wantOrder)
	}

	// Round trip: unseal.
	back, _, opaque, err := unsealProperties(c, sealed, "")
	if err != nil {
		t.Fatalf("unseal: %v", err)
	}
	if len(opaque) != 0 {
		t.Errorf("no opaque expected, got %v", opaque)
	}
	if back["title"] != "Hello" {
		t.Errorf("title lost: %v", back["title"])
	}
	if back["description"] != "top secret" {
		t.Errorf("description not restored: %v", back["description"])
	}
	// Encryption metadata stripped from unseal output.
	if _, ok := back["_encryption"]; ok {
		t.Error("unseal left _encryption in output")
	}
}

func TestSealUnseal_MultiRecipient_BobCanDecrypt(t *testing.T) {
	// Seal with alice's pubkey + bob's pubkey (both in engineering),
	// decrypt using bob's keyring.
	f := newFixture(t)
	sealer := f.cryptoAs("alice")
	sealed, _, _, err := sealProperties(
		sealer, "ticket",
		map[string]any{"description": "shared secret"},
		"",
		[]string{"description"})
	if err != nil {
		t.Fatal(err)
	}
	// Now unseal as bob.
	opener := f.cryptoAs("bob")
	back, _, _, err := unsealProperties(opener, sealed, "")
	if err != nil {
		t.Fatalf("unseal as bob: %v", err)
	}
	if back["description"] != "shared secret" {
		t.Errorf("bob decrypt = %v, want shared secret", back["description"])
	}
}

func TestSealUnseal_WrongKey_Opaque(t *testing.T) {
	// Seal with engineering:[alice,bob], try to decrypt as eve.
	f := newFixture(t)
	sealer := f.cryptoAs("alice")
	sealed, _, _, err := sealProperties(
		sealer, "ticket",
		map[string]any{"description": "secret"},
		"",
		[]string{"description"})
	if err != nil {
		t.Fatal(err)
	}
	opener := f.cryptoAs("eve")
	back, _, opaque, err := unsealProperties(opener, sealed, "")
	if err != nil {
		t.Fatalf("unseal as eve: %v", err)
	}
	// Eve cannot decrypt → property surfaces as Opaque.
	if len(opaque) != 1 || !opaque["description"] {
		t.Errorf("expected description in opaque set, got %v", opaque)
	}
	op, ok := back["description"].(encryption.Opaque)
	if !ok {
		t.Fatalf("description = %T, want encryption.Opaque", back["description"])
	}
	if op.Len() == 0 {
		t.Error("opaque len 0")
	}
}

func TestSealProperties_OpaquePresent_Refuses(t *testing.T) {
	// Slice 3 strict semantics: any Opaque value at write time forces
	// refusal. Re-emitting an Opaque under a fresh envelope would
	// produce a file whose ciphertext was sealed under an old data
	// key that the new envelope no longer carries — no one could
	// decrypt it. Users without decryption capability cannot write.
	f := newFixture(t)

	// Alice seals.
	sealer := f.cryptoAs("alice")
	sealed, order, _, err := sealProperties(
		sealer, "ticket",
		map[string]any{"title": "hi", "description": "secret"},
		"",
		[]string{"title", "description"})
	if err != nil {
		t.Fatal(err)
	}

	// Eve reads → gets Opaque for description.
	evesCrypto := f.cryptoAs("eve")
	readByEve, _, opaque, err := unsealProperties(evesCrypto, sealed, "")
	if err != nil {
		t.Fatal(err)
	}
	if !opaque["description"] {
		t.Fatal("expected Opaque for description when eve reads")
	}

	// Eve tries to write (even unchanged): must be refused with
	// OpaqueWrite. The property name in the error identifies the
	// offending field.
	_, _, _, err = sealProperties(evesCrypto, "ticket", readByEve, "", order) //nolint:dogsled // test asserts error-only contract
	if err == nil {
		t.Fatal("expected OpaqueWrite refusal, got nil")
	}
	var ee *EncryptionError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v, want EncryptionError", err)
	}
	if ee.Kind != ErrKindOpaqueWrite {
		t.Errorf("Kind = %s, want %s", ee.Kind, ErrKindOpaqueWrite)
	}
	if ee.Property != "description" {
		t.Errorf("Property = %q, want description", ee.Property)
	}
}

func TestSealUnseal_MultipleProperties_SameGroup(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	c.propGroups["ticket"]["notes"] = "engineering"

	props := map[string]any{
		"title":       "Hello",
		"description": "secret A",
		"notes":       "secret B",
	}
	order := []string{"title", "description", "notes"}
	sealed, _, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}
	back, _, _, err := unsealProperties(c, sealed, "")
	if err != nil {
		t.Fatal(err)
	}
	if back["description"] != "secret A" || back["notes"] != "secret B" {
		t.Errorf("round-trip lost values: %v / %v", back["description"], back["notes"])
	}
}

// --- tests: tamper detection ------------------------------------------

func TestUnsealProperties_TamperedCiphertext(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	sealed, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": "top secret"},
		"",
		[]string{"description"})
	if err != nil {
		t.Fatal(err)
	}
	// Tamper: mutate the last char of the base64 ciphertext.
	b64 := sealed["_enc_v1_description"].(string)
	if len(b64) < 4 {
		t.Fatal("ciphertext too short for tamper test")
	}
	// Flip a character in the middle (out of padding). Base64 alphabet
	// is predictable — swap 'A' ↔ 'B'.
	mid := len(b64) / 2
	mutated := b64[:mid] + swapChar(b64[mid:mid+1]) + b64[mid+1:]
	sealed["_enc_v1_description"] = mutated

	_, _, opaque, err := unsealProperties(c, sealed, "")
	// Tamper produces either CorruptedFile or an Opaque value, never
	// correct plaintext. Check we didn't recover the plaintext.
	if err == nil {
		if !opaque["description"] {
			t.Error("tampered ciphertext decoded successfully without opaque — leak")
		}
		return
	}
	var ee *EncryptionError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v, want EncryptionError", err)
	}
	if ee.Kind != ErrKindCorruptedFile {
		t.Errorf("Kind = %s, want %s", ee.Kind, ErrKindCorruptedFile)
	}
}

// swapChar toggles one base64-alphabet character to simulate tamper.
func swapChar(s string) string {
	if s == "" {
		return s
	}
	c := s[0]
	switch {
	case c >= 'A' && c < 'Z':
		c++
	case c == 'Z':
		c = 'a'
	case c >= 'a' && c < 'z':
		c++
	case c == 'z':
		c = '0'
	case c >= '0' && c < '9':
		c++
	default:
		c = 'A'
	}
	return string(c)
}

func TestUnsealProperties_TamperedEnvelopeWrap(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	sealed, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": "secret"},
		"",
		[]string{"description"})
	if err != nil {
		t.Fatal(err)
	}
	// Tamper with alice's wrapped blob.
	envMap := sealed["_encryption"].(map[string]any)
	groups := envMap["data_keys"].(map[string]any)
	eng := groups["engineering"].(map[string]any)
	alice := eng["alice"].(string)
	if len(alice) < 4 {
		t.Fatal("wrapped blob too short")
	}
	eng["alice"] = alice[:2] + swapChar(alice[2:3]) + alice[3:]

	_, _, opaque, err := unsealProperties(c, sealed, "")
	// Tampered wrapped blob: UnwrapKey fails. Our test fake calls
	// encryption.UnwrapKey which returns ErrDecrypt/ErrBadBlob → we
	// classify as CorruptedFile.
	if err == nil {
		// Edge: if bob had also been tried but failed, we'd get
		// ErrNoMatchingKey → Opaque. Since our fake only checks our
		// own identity, alice is tried, fails, and we surface
		// CorruptedFile. Accept Opaque as well to be robust.
		if !opaque["description"] {
			t.Error("tampered wrap decoded or left partial-decrypt unsupported")
		}
		return
	}
	var ee *EncryptionError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v, want EncryptionError", err)
	}
	if ee.Kind != ErrKindCorruptedFile {
		t.Errorf("Kind = %s, want %s", ee.Kind, ErrKindCorruptedFile)
	}
}

// --- tests: envelope shape / determinism ------------------------------

// TestUnsealProperties_RejectsUnsupportedKeyVersion is C2 regression:
// a future v2 envelope format must NOT be silently parsed as v1
// (which would produce garbage data keys). Also reject non-numeric
// key_version types.
func TestUnsealProperties_RejectsUnsupportedKeyVersion(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")

	// First produce a valid sealed map, then mutate its key_version.
	sealed, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": "s"},
		"",
		[]string{"description"})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		kv   any
	}{
		{"future version", 2},
		{"string version", "1"},
		{"bool version", true},
		{"nil version", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := sealed[encryptionKey].(map[string]any)
			env["key_version"] = tc.kv
			_, _, _, err := unsealProperties(c, sealed, "")
			var ee *EncryptionError
			if !errors.As(err, &ee) {
				t.Fatalf("err = %v, want EncryptionError", err)
			}
			if ee.Kind != ErrKindCorruptedFile {
				t.Errorf("Kind = %s, want CorruptedFile", ee.Kind)
			}
		})
	}
}

func TestSealProperties_EnvelopeShape(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	sealed, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": "s", "secret": "t"},
		"",
		[]string{"description", "secret"})
	if err != nil {
		t.Fatal(err)
	}
	env := sealed["_encryption"].(map[string]any)
	if env["key_version"] != 1 {
		t.Errorf("key_version = %v, want 1", env["key_version"])
	}
	dk := env["data_keys"].(map[string]any)
	// Expect both groups: engineering (description) and exec (secret).
	if _, ok := dk["engineering"]; !ok {
		t.Error("engineering group missing from envelope")
	}
	if _, ok := dk["exec"]; !ok {
		t.Error("exec group missing from envelope")
	}
	// engineering has alice + bob, exec has bob only (per fakeCrypto
	// default groups).
	engIDs := identitiesIn(dk["engineering"].(map[string]any))
	if !reflect.DeepEqual(engIDs, []string{"alice", "bob"}) {
		t.Errorf("engineering identities = %v", engIDs)
	}
	execIDs := identitiesIn(dk["exec"].(map[string]any))
	if !reflect.DeepEqual(execIDs, []string{"bob"}) {
		t.Errorf("exec identities = %v", execIDs)
	}
}

func identitiesIn(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestSealProperties_FreshDataKeyPerWrite(t *testing.T) {
	// Two consecutive writes of the same inputs must produce
	// different ciphertext (fresh data key per write).
	f := newFixture(t)
	c := f.cryptoAs("alice")
	props := map[string]any{"description": "same"}
	order := []string{"description"}
	s1, _, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}
	s2, _, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}
	if s1["_enc_v1_description"] == s2["_enc_v1_description"] {
		t.Error("two writes produced identical ciphertext — data key reuse?")
	}
}

// --- tests: empty string ---------------------------------------------

func TestSealUnseal_EmptyString(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	sealed, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": ""},
		"",
		[]string{"description"})
	if err != nil {
		t.Fatal(err)
	}
	back, _, _, err := unsealProperties(c, sealed, "")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := back["description"]
	if !ok {
		t.Fatal("description missing after round trip")
	}
	if v != "" {
		t.Errorf("description = %v, want empty string", v)
	}
}

// --- tests: body encryption ------------------------------------------

func TestSealUnseal_BodyRoundTrip(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	c.bodyGroups["ticket"] = "engineering"

	sealed, _, sealedBody, err := sealProperties(
		c, "ticket",
		map[string]any{"title": "hi"},
		"body text here",
		[]string{"title"})
	if err != nil {
		t.Fatal(err)
	}
	if sealedBody != "" {
		t.Errorf("sealed body should be empty (body moved to frontmatter), got %q", sealedBody)
	}
	if _, ok := sealed["_encrypted_body"]; !ok {
		t.Fatal("missing _encrypted_body in sealed map")
	}

	_, body, _, err := unsealProperties(c, sealed, "")
	if err != nil {
		t.Fatal(err)
	}
	if body != "body text here" {
		t.Errorf("body = %q, want %q", body, "body text here")
	}
}

// --- tests: unknown group / recipient ---------------------------------

func TestSealProperties_UnknownGroup(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	c.propGroups["ticket"]["description"] = "ghost" // not in c.groups

	//nolint:dogsled // test asserts error-only contract
	_, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": "x"},
		"",
		[]string{"description"})
	if err == nil {
		t.Fatal("expected error for unknown group")
	}
	var ee *EncryptionError
	if !errors.As(err, &ee) || ee.Kind != ErrKindUnknownGroup {
		t.Errorf("err = %v, want UnknownGroup", err)
	}
}

func TestSealProperties_UnknownRecipient(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	c.groups["engineering"] = []string{"alice", "ghost"} // ghost has no pubkey

	//nolint:dogsled // test asserts error-only contract
	_, _, _, err := sealProperties(
		c, "ticket",
		map[string]any{"description": "x"},
		"",
		[]string{"description"})
	if err == nil {
		t.Fatal("expected error for unknown recipient")
	}
	var ee *EncryptionError
	if !errors.As(err, &ee) || ee.Kind != ErrKindUnknownRecipient {
		t.Errorf("err = %v, want UnknownRecipient", err)
	}
}

// --- tests: envelope determinism (non-ciphertext fields) -------------

// TestSealProperties_OneKeyPerGroupPerWrite pins down the
// "fresh data key per write per group" invariant that keeps the
// AES-GCM 2^32 message-per-key bound unreachable. Two sequential
// writes of the same entity produce different ciphertext even for
// identical plaintext — proving each write generates fresh data
// keys and doesn't reuse an earlier one.
func TestSealProperties_OneKeyPerGroupPerWrite(t *testing.T) {
	f := newFixture(t)
	c := f.cryptoAs("alice")
	props := map[string]any{"description": "same value"}
	order := []string{"description"}

	s1, _, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}
	s2, _, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}
	if s1["_enc_v1_description"] == s2["_enc_v1_description"] {
		t.Fatal("identical ciphertext across writes — data-key reuse (hits 2^32 bound over time)")
	}
	// The envelope's wrapped data key also differs (new data key
	// means new wrapped blob).
	env1 := s1["_encryption"].(map[string]any)
	env2 := s2["_encryption"].(map[string]any)
	g1 := env1["data_keys"].(map[string]any)["engineering"].(map[string]any)
	g2 := env2["data_keys"].(map[string]any)["engineering"].(map[string]any)
	if g1["alice"] == g2["alice"] {
		t.Fatal("envelope wrap identical across writes — data-key reuse")
	}
}

// TestSealUnseal_MultiGroup_IsolationWithinSameFile is the missing
// coverage that the end-to-end demo exposed: with one data key per
// group (not per file), a recipient of group A CANNOT decrypt
// properties sealed for group B, even though both envelopes live in
// the same file. Under the earlier single-per-file-data-key design
// this test would have revealed the cross-group leak.
func TestSealUnseal_MultiGroup_IsolationWithinSameFile(t *testing.T) {
	f := newFixture(t)
	// description → engineering ([alice, bob])
	// secret      → exec ([bob])
	//
	// alice is NOT in exec. She should see description cleartext
	// but secret as Opaque.
	c := f.cryptoAs("alice")

	props := map[string]any{
		"description": "eng-plaintext",
		"secret":      "exec-plaintext",
	}
	order := []string{"description", "secret"}
	sealed, _, _, err := sealProperties(c, "ticket", props, "", order)
	if err != nil {
		t.Fatal(err)
	}
	// The envelope should have two groups, each with its own data key
	// wrapped. Different ciphertext for description and secret.
	b1 := sealed["_enc_v1_description"]
	b2 := sealed["_enc_v1_secret"]
	if b1 == b2 {
		t.Fatal("two properties sealed with same ciphertext — data-key reuse across groups?")
	}

	// Unseal as alice.
	back, _, opaque, err := unsealProperties(c, sealed, "")
	if err != nil {
		t.Fatal(err)
	}
	if back["description"] != "eng-plaintext" {
		t.Errorf("description should decrypt, got %v", back["description"])
	}
	if _, isOpaque := back["secret"].(encryption.Opaque); !isOpaque {
		t.Fatalf("secret MUST be Opaque for alice (not in exec), got %T = %v",
			back["secret"], back["secret"])
	}
	if !opaque["secret"] {
		t.Error("opaque set missing 'secret'")
	}

	// Unseal as bob (member of both groups): both decrypt.
	bobC := f.cryptoAs("bob")
	backBob, _, opaqueBob, err := unsealProperties(bobC, sealed, "")
	if err != nil {
		t.Fatal(err)
	}
	if backBob["description"] != "eng-plaintext" {
		t.Errorf("bob description = %v", backBob["description"])
	}
	if backBob["secret"] != "exec-plaintext" {
		t.Errorf("bob secret = %v", backBob["secret"])
	}
	if len(opaqueBob) != 0 {
		t.Errorf("bob should see zero Opaque values, got %v", opaqueBob)
	}
}

func TestSealProperties_EnvelopeGroupOrder(t *testing.T) {
	// Two writes must yield the same set of groups in data_keys.
	f := newFixture(t)
	c := f.cryptoAs("alice")
	props := map[string]any{"description": "a", "secret": "b"}
	order := []string{"description", "secret"}
	s1, _, _, _ := sealProperties(c, "ticket", props, "", order) //nolint:dogsled // test compares sealed maps only
	s2, _, _, _ := sealProperties(c, "ticket", props, "", order) //nolint:dogsled // test compares sealed maps only

	env1 := s1["_encryption"].(map[string]any)["data_keys"].(map[string]any)
	env2 := s2["_encryption"].(map[string]any)["data_keys"].(map[string]any)

	g1 := sortedGroups(env1)
	g2 := sortedGroups(env2)
	if !reflect.DeepEqual(g1, g2) {
		t.Errorf("group set differs: %v vs %v", g1, g2)
	}
}

func sortedGroups(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
