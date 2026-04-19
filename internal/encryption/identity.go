package encryption

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
)

// Recipient is a public key that a sealed blob can be encrypted for.
// It wraps age.Recipient so callers don't need to import age directly.
type Recipient interface {
	ageRecipient() age.Recipient
	// String returns the canonical age public-key encoding
	// ("age1pq1..."). Safe to print, log, and commit.
	String() string
}

// Identity is a private key used to decrypt sealed blobs. The secret
// scalar is never exposed through String or MarshalJSON; see
// TestIdentity_NoSecretLeak.
type Identity interface {
	ageIdentity() age.Identity
	// PublicRecipient returns the Recipient corresponding to this
	// Identity, for membership checks against a keyring.
	PublicRecipient() Recipient
}

// hybridRecipient wraps *age.HybridRecipient with a stable interface.
type hybridRecipient struct {
	r *age.HybridRecipient
}

func (r *hybridRecipient) ageRecipient() age.Recipient { return r.r }
func (r *hybridRecipient) String() string              { return r.r.String() }

// hybridIdentity wraps *age.HybridIdentity. String intentionally
// redacts the secret bytes.
type hybridIdentity struct {
	i *age.HybridIdentity
}

func (i *hybridIdentity) ageIdentity() age.Identity { return i.i }
func (i *hybridIdentity) PublicRecipient() Recipient {
	return &hybridRecipient{r: i.i.Recipient()}
}

// String returns a fixed redacted marker. We deliberately do NOT
// return age.HybridIdentity.String() (the AGE-SECRET-KEY-PQ-1... form)
// because Identity values flow through logs and error messages in
// calling code; accidentally printing them must not leak the key.
func (i *hybridIdentity) String() string { return "<redacted age identity>" }

// MarshalJSON mirrors String: refuse to serialize the secret.
func (i *hybridIdentity) MarshalJSON() ([]byte, error) {
	return []byte(`"<redacted age identity>"`), nil
}

// GenerateIdentity returns a fresh hybrid (post-quantum) age identity.
func GenerateIdentity() (Identity, error) {
	id, err := age.GenerateHybridIdentity()
	if err != nil {
		return nil, fmt.Errorf("encryption: generate identity: %w", err)
	}
	return &hybridIdentity{i: id}, nil
}

// ParseRecipient parses an age hybrid public-key string ("age1pq1...")
// into a Recipient. Accepts one recipient per input; rejects empty
// input.
func ParseRecipient(s string) (Recipient, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("encryption: empty recipient string")
	}
	r, err := age.ParseHybridRecipient(s)
	if err != nil {
		return nil, fmt.Errorf("encryption: parse recipient: %w", err)
	}
	return &hybridRecipient{r: r}, nil
}

// ParseIdentity parses an age hybrid private-key string
// ("AGE-SECRET-KEY-PQ-1...") into an Identity.
func ParseIdentity(s string) (Identity, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("encryption: empty identity string")
	}
	id, err := age.ParseHybridIdentity(s)
	if err != nil {
		return nil, fmt.Errorf("encryption: parse identity: %w", err)
	}
	return &hybridIdentity{i: id}, nil
}

// ReadIdentity reads a single age identity from r. Equivalent to
// reading r to a string and passing it to ParseIdentity, except it
// uses age.ParseIdentities internally so future-format compatibility
// (comments, blank lines) comes for free.
func ReadIdentity(r io.Reader) (Identity, error) {
	ids, err := age.ParseIdentities(r)
	if err != nil {
		return nil, fmt.Errorf("encryption: parse identity: %w", err)
	}
	if len(ids) == 0 {
		return nil, errors.New("encryption: no identity found in input")
	}
	if len(ids) > 1 {
		return nil, errors.New("encryption: multiple identities in input (expected one)")
	}
	h, ok := ids[0].(*age.HybridIdentity)
	if !ok {
		return nil, fmt.Errorf("encryption: unsupported identity kind %T (want Hybrid)", ids[0])
	}
	return &hybridIdentity{i: h}, nil
}

// recipientsAsAge extracts the underlying age.Recipient from each
// Recipient. Exists so Seal can pass the right types to age.Encrypt.
func recipientsAsAge(rs []Recipient) []age.Recipient {
	out := make([]age.Recipient, len(rs))
	for i, r := range rs {
		out[i] = r.ageRecipient()
	}
	return out
}
