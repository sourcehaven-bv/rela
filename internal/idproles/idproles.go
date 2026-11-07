// Package idproles keeps the role claims an identity proxy asserted for each
// user, so they can apply where no request carries them (TKT-IZL6NX8).
//
// A verified JWT carries a `roles` claim, and `asserted_role_assignments` in
// acl.yaml maps it to ACL roles for that request. A background job has no JWT.
// The IdP webhook receiver therefore stores the IdP role names its action
// fetched for the subject the signed webhook named, and a scheduler `for_each`
// child or a Lua role query reads them back here.
//
// # What is stored
//
// One [Record] per subject: the IdP's role names, NOT rela roles. They are
// mapped through `asserted_role_assignments` at the point of use, exactly as a
// JWT claim is, so a stored value can never name a rela role the operator did
// not map.
//
// A record also names the user entity the subject resolved to when the
// webhook stored it. A reader applies the roles only to that entity, so
// moving the subject to another entity (by editing its principal_property
// value) carries no roles until a new verified webhook binds them again.
//
// # Where it is stored
//
// One JSON document in the state KV, rewritten under a lock. That is runtime
// state outside the graph: no entity write path reaches it, so a user who may
// edit a person entity cannot grant that person a role. On postgres the KV is
// the tenant schema's `state_kv` table and the lock is a schema-scoped
// advisory lock, so several rela-server processes share one record set and
// never lose each other's writes. On the single-process tiers both are local.
//
// # Who may write
//
// Only the webhook receiver, after it verified the webhook signature, for the
// subject the webhook names. This package does not check that; it trusts its
// caller the way internal/acl trusts a Principal. The receiver holds the
// subject ([Store.Hold]) from before its action fetches the roles until it
// stored them, so an older fetch cannot overwrite a newer one.
package idproles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// StateKey is where the document lives in the state KV.
	StateKey = "idp-roles.json"
	// LockKey names the lock every document read and write takes.
	LockKey = "idp-roles"
	// SubjectLockPrefix prefixes the key [Store.Hold] takes for a subject.
	SubjectLockPrefix = "idp-roles/subject/"

	// MaxRoles bounds the roles stored for one subject.
	MaxRoles = 64
	// MaxNameLen bounds a role name, a subject and an org id, in bytes.
	MaxNameLen = 256
)

// KV is the state store the document lives in. state.KV satisfies it; a
// missing key is reported as an error satisfying os.IsNotExist.
type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
}

// Locker serializes writers of the document across every process that shares
// the KV. lock.Locker satisfies it.
type Locker interface {
	Acquire(ctx context.Context, key string) (release func(), err error)
}

// Record is the stored role set of one subject. The document keys it by the
// subject.
type Record struct {
	// EntityID is the user entity the subject resolved to when the roles
	// were stored.
	EntityID  string    `json:"entity_id"`
	OrgID     string    `json:"org_id,omitempty"`
	Roles     []string  `json:"roles"`
	UpdatedAt time.Time `json:"updated_at"`
}

// document is the stored JSON shape.
type document struct {
	Version  int               `json:"version"`
	Subjects map[string]Record `json:"subjects"`
}

const documentVersion = 1

// Store reads and writes the role document.
type Store struct {
	kv     KV
	locker Locker
	now    func() time.Time
}

// New returns a Store over kv, serializing writes with locker.
//
// Nil: both rejected. A nil locker would let two webhooks lose each other's
// writes, and a lost write can be a lost revocation.
func New(kv KV, locker Locker) (*Store, error) {
	if kv == nil {
		return nil, errors.New("idproles: state KV must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("idproles: locker must be non-nil")
	}
	return &Store{kv: kv, locker: locker, now: time.Now}, nil
}

// Hold takes the lock of one subject and returns its release. The webhook
// receiver holds it across its action and [Store.Set], so two deliveries for
// one subject store in the order they fetched.
func (s *Store) Hold(ctx context.Context, subject string) (func(), error) {
	if err := validName("subject", subject); err != nil {
		return nil, err
	}
	release, err := s.locker.Acquire(ctx, SubjectLockPrefix+subject)
	if err != nil {
		return nil, fmt.Errorf("idproles: lock subject: %w", err)
	}
	return release, nil
}

// Set replaces the stored roles of subject, bound to the user entity
// entityID. An empty roles list removes the subject's record, which is how a
// webhook revokes; entityID may then be empty.
//
// Roles are trimmed and deduplicated. A name that is empty after trimming,
// longer than [MaxNameLen] or contains a control character is refused, as is a
// list longer than [MaxRoles]. Nothing is stored on a refusal.
func (s *Store) Set(ctx context.Context, subject, orgID, entityID string, roles []string) error {
	if err := validName("subject", subject); err != nil {
		return err
	}
	if len(roles) > 0 {
		if err := validName("entity id", entityID); err != nil {
			return err
		}
	}
	if orgID != "" {
		if err := validName("org id", orgID); err != nil {
			return err
		}
	}
	clean, err := NormalizeRoles(roles)
	if err != nil {
		return err
	}
	release, err := s.locker.Acquire(ctx, LockKey)
	if err != nil {
		return fmt.Errorf("idproles: lock: %w", err)
	}
	defer release()
	doc, err := s.load(ctx)
	if err != nil {
		return err
	}
	if len(clean) == 0 {
		if _, ok := doc.Subjects[subject]; !ok {
			return nil
		}
		delete(doc.Subjects, subject)
	} else {
		doc.Subjects[subject] = Record{EntityID: entityID, OrgID: orgID, Roles: clean, UpdatedAt: s.now().UTC()}
	}
	return s.flush(ctx, doc)
}

// RolesFor returns the stored roles of subject when they were stored for the
// user entity entityID, else nil.
func (s *Store) RolesFor(ctx context.Context, subject, entityID string) ([]string, error) {
	doc, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	rec, ok := doc.Subjects[subject]
	if !ok || rec.EntityID == "" || rec.EntityID != entityID {
		return nil, nil
	}
	return slices.Clone(rec.Roles), nil
}

// Holder is a subject with stored roles and the entity they were stored for.
type Holder struct {
	Subject  string
	EntityID string
}

// HoldersWithAny returns the holders whose stored roles include any of
// claims, sorted by subject.
func (s *Store) HoldersWithAny(ctx context.Context, claims []string) ([]Holder, error) {
	if len(claims) == 0 {
		return nil, nil
	}
	doc, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	var out []Holder
	for subject, rec := range doc.Subjects {
		if rec.EntityID == "" {
			continue
		}
		if slices.ContainsFunc(rec.Roles, func(r string) bool { return slices.Contains(claims, r) }) {
			out = append(out, Holder{Subject: subject, EntityID: rec.EntityID})
		}
	}
	slices.SortFunc(out, func(a, b Holder) int { return strings.Compare(a.Subject, b.Subject) })
	return out, nil
}

// read loads the document under the lock. Reads lock too, so a reader never
// holds the file open while a writer replaces it on Windows.
func (s *Store) read(ctx context.Context) (*document, error) {
	release, err := s.locker.Acquire(ctx, LockKey)
	if err != nil {
		return nil, fmt.Errorf("idproles: lock: %w", err)
	}
	defer release()
	return s.load(ctx)
}

// load reads and decodes the document; the caller holds the lock. A missing
// document is empty. A damaged one is an error, never an empty document:
// treating it as empty would let the next write erase every other subject.
func (s *Store) load(ctx context.Context) (*document, error) {
	raw, err := s.kv.Get(ctx, StateKey)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
			return &document{Version: documentVersion, Subjects: map[string]Record{}}, nil
		}
		return nil, fmt.Errorf("idproles: read %s: %w", StateKey, err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		// The decode error may quote the document, so it is not wrapped.
		return nil, fmt.Errorf("idproles: %s is damaged; fix or remove it", StateKey)
	}
	if doc.Version != documentVersion {
		return nil, fmt.Errorf("idproles: %s has unknown version %d", StateKey, doc.Version)
	}
	if doc.Subjects == nil {
		doc.Subjects = map[string]Record{}
	}
	return &doc, nil
}

func (s *Store) flush(ctx context.Context, doc *document) error {
	raw, err := json.Marshal(doc)
	if err != nil { // coverage-ignore: unreachable: the document holds only strings and times
		return fmt.Errorf("idproles: encode: %w", err)
	}
	if err := s.kv.Put(ctx, StateKey, raw); err != nil {
		return fmt.Errorf("idproles: write %s: %w", StateKey, err)
	}
	return nil
}

// NormalizeRoles trims, validates and deduplicates roles, keeping first-seen
// order. See [Store.Set] for the rules.
func NormalizeRoles(roles []string) ([]string, error) {
	if len(roles) > MaxRoles {
		return nil, fmt.Errorf("idproles: %d roles exceed the limit of %d", len(roles), MaxRoles)
	}
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if err := validName("role", r); err != nil {
			return nil, err
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out, nil
}

// validName refuses an empty, over-long, invalid UTF-8 or control-character
// value. The value itself is not quoted in the error: it came from outside.
func validName(what, v string) error {
	switch {
	case v == "":
		return fmt.Errorf("idproles: %s must not be empty", what)
	case len(v) > MaxNameLen:
		return fmt.Errorf("idproles: %s is longer than %d bytes", what, MaxNameLen)
	case !utf8.ValidString(v):
		return fmt.Errorf("idproles: %s is not valid UTF-8", what)
	case strings.IndexFunc(v, unicode.IsControl) >= 0:
		return fmt.Errorf("idproles: %s contains a control character", what)
	}
	return nil
}
