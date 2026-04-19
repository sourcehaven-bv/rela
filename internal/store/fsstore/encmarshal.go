package fsstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// On-disk key-prefix constants. Versioned in the prefix so a future
// format bump produces structurally distinguishable files.
const (
	encKeyPrefix     = "_enc_v1_"
	encryptionKey    = "_encryption"
	encryptedBodyKey = "_encrypted_body"
)

// stripEncKey returns (propName, true) if key has the _enc_v1_ prefix.
func stripEncKey(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, encKeyPrefix)
	return rest, ok
}

// applyEncKey wraps a plain property name with the _enc_v1_ prefix.
func applyEncKey(propName string) string { return encKeyPrefix + propName }

// encryptionBlock is the in-memory representation of the _encryption
// frontmatter block. It carries the wrapped per-recipient data keys,
// organized by group. The data key itself is never serialized.
type encryptionBlock struct {
	keyVersion int
	// dataKeys maps group → identity → wrapped blob (raw bytes).
	dataKeys map[string]map[string][]byte
}

// asFrontmatter renders the encryption block to the map[string]any
// shape used by formatDocumentOrdered. Group names and identity names
// are sorted for deterministic output.
func (b *encryptionBlock) asFrontmatter() map[string]any {
	out := map[string]any{
		"key_version": b.keyVersion,
	}
	groups := make(map[string]any, len(b.dataKeys))
	for g, ids := range b.dataKeys {
		wrapMap := make(map[string]any, len(ids))
		for id, wrapped := range ids {
			wrapMap[id] = base64.StdEncoding.EncodeToString(wrapped)
		}
		groups[g] = wrapMap
	}
	out["data_keys"] = groups
	return out
}

// parseEncryptionBlock converts the raw frontmatter value (as returned
// by yaml.Unmarshal into map[string]any) into an encryptionBlock.
// Callers must pre-check raw != nil before calling — a nil raw is a
// programming error (no encryption block means you shouldn't be
// parsing one).
func parseEncryptionBlock(raw any) (*encryptionBlock, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("fsstore: _encryption must be a map, got %T", raw)
	}
	blk := &encryptionBlock{
		keyVersion: 1,
		dataKeys:   make(map[string]map[string][]byte),
	}
	// key_version (optional, defaults to 1).
	if v, hasKV := m["key_version"]; hasKV {
		switch kv := v.(type) {
		case int:
			blk.keyVersion = kv
		case int64:
			blk.keyVersion = int(kv)
		case float64:
			blk.keyVersion = int(kv)
		}
	}
	// data_keys: group → identity → base64 wrap.
	dkRaw, ok := m["data_keys"]
	if !ok {
		return blk, nil
	}
	dkMap, ok := dkRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("fsstore: _encryption.data_keys must be a map, got %T", dkRaw)
	}
	for group, inner := range dkMap {
		innerMap, ok := inner.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("fsstore: _encryption.data_keys.%s must be a map, got %T", group, inner)
		}
		wraps := make(map[string][]byte, len(innerMap))
		for identity, b64 := range innerMap {
			s, ok := b64.(string)
			if !ok {
				return nil, fmt.Errorf("fsstore: _encryption.data_keys.%s.%s must be a string, got %T", group, identity, b64)
			}
			raw, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, fmt.Errorf("fsstore: _encryption.data_keys.%s.%s: %w", group, identity, err)
			}
			wraps[identity] = raw
		}
		blk.dataKeys[group] = wraps
	}
	return blk, nil
}

// sealProperties takes the entity's frontmatter (plain property map,
// keyed by property name, values as Go types) and the entity type,
// plus the Crypto policy, and produces:
//
//   - A new frontmatter map with encrypted properties renamed to
//     _enc_v1_<propName> and their values replaced by base64-encoded
//     ciphertext strings, plus an injected _encryption block.
//   - A rewritten key order with encrypted slots preserving their
//     original position (only the key name changes) and _encryption
//     appended at the end.
//   - An error if sealing is impossible (unknown group, unknown
//     recipient, opaque-write violation).
//
// Body handling: if the type's body is encrypted, the body content
// (passed as rawBody) must be sealed and the resulting ciphertext
// emitted under the _encrypted_body key in the returned frontmatter.
// In that case sealedBody returns empty — the markdown body section
// must be empty on disk. If rawBody is non-empty AND the body is
// declared encrypted, we seal normally (first write). If an already
// sealed _encrypted_body exists in an incoming frontmatter via opaque
// flow, callers should not pass it through sealProperties; they
// should re-emit verbatim. (Slice 3 keeps this path simple: fsstore
// always decrypts on read, and writes go through sealProperties with
// cleartext input.)
//
// entityType must match a type known to Crypto; unknown types have
// every property cleartext (no policy = no encryption).
//
// If crypto is nil, sealProperties is a no-op: returns props
// unchanged and order unchanged.
func sealProperties(
	crypto Crypto,
	entityType string,
	props map[string]any,
	rawBody string,
	order []string,
) (frontmatter map[string]any, newOrder []string, sealedBody string, err error) {
	if crypto == nil {
		return props, order, rawBody, nil
	}

	plan := discoverEncryption(crypto, entityType, props)
	if !plan.hasWork() {
		return props, order, rawBody, nil
	}
	if opaqueErr := refuseOpaqueWrites(props); opaqueErr != nil {
		return nil, nil, "", opaqueErr
	}

	dataKey, err := encryption.NewDataKey()
	if err != nil {
		return nil, nil, "", fmt.Errorf("fsstore: generate data key: %w", err)
	}

	envelope, envErr := buildEnvelope(crypto, plan.groupsNeeded, dataKey)
	if envErr != nil {
		return nil, nil, "", envErr
	}

	frontmatter, newOrder, fmErr := sealFrontmatter(props, order, plan.propGroups, dataKey)
	if fmErr != nil {
		return nil, nil, "", fmErr
	}

	sealedBody, newOrder, bodyErr := sealBodyIfNeeded(rawBody, dataKey, plan.bodyEncrypted, frontmatter, newOrder)
	if bodyErr != nil {
		return nil, nil, "", bodyErr
	}

	frontmatter[encryptionKey] = envelope.asFrontmatter()
	newOrder = append(newOrder, encryptionKey)
	return frontmatter, newOrder, sealedBody, nil
}

// encryptionPlan describes what needs to happen for one write.
type encryptionPlan struct {
	propGroups    map[string]string   // property name → group name
	groupsNeeded  map[string]struct{} // set of all groups that need envelope entries
	bodyEncrypted bool
}

func (p *encryptionPlan) hasWork() bool { return len(p.groupsNeeded) > 0 }

func discoverEncryption(crypto Crypto, entityType string, props map[string]any) *encryptionPlan {
	p := &encryptionPlan{
		propGroups:   make(map[string]string, len(props)),
		groupsNeeded: make(map[string]struct{}),
	}
	for name := range props {
		if g, encrypted := crypto.PropertyGroup(entityType, name); encrypted {
			p.propGroups[name] = g
			p.groupsNeeded[g] = struct{}{}
		}
	}
	if g, enc := crypto.BodyGroup(entityType); enc {
		p.bodyEncrypted = true
		p.groupsNeeded[g] = struct{}{}
	}
	return p
}

func refuseOpaqueWrites(props map[string]any) error {
	for name, v := range props {
		if _, ok := v.(encryption.Opaque); ok {
			return &EncryptionError{Kind: ErrKindOpaqueWrite, Property: name}
		}
	}
	return nil
}

func buildEnvelope(crypto Crypto, groups map[string]struct{}, dataKey []byte) (*encryptionBlock, error) {
	envelope := &encryptionBlock{
		keyVersion: 1,
		dataKeys:   make(map[string]map[string][]byte, len(groups)),
	}
	for g := range groups {
		ids, ok := crypto.Recipients(g)
		if !ok {
			return nil, &EncryptionError{
				Kind:  ErrKindUnknownGroup,
				Cause: fmt.Errorf("group %q", g),
			}
		}
		wraps, err := wrapForGroup(crypto, g, ids, dataKey)
		if err != nil {
			return nil, err
		}
		envelope.dataKeys[g] = wraps
	}
	return envelope, nil
}

func wrapForGroup(crypto Crypto, group string, ids []string, dataKey []byte) (map[string][]byte, error) {
	wraps := make(map[string][]byte, len(ids))
	for _, id := range ids {
		pub, ok := crypto.Recipient(id)
		if !ok {
			return nil, &EncryptionError{
				Kind:  ErrKindUnknownRecipient,
				Cause: fmt.Errorf("identity %q (group %q)", id, group),
			}
		}
		wrapped, err := encryption.WrapKey(dataKey, pub)
		if err != nil {
			return nil, fmt.Errorf("fsstore: wrap key for %s/%s: %w", group, id, err)
		}
		wraps[id] = wrapped
	}
	return wraps, nil
}

// sealFrontmatter applies key renames + value sealing to props,
// preserving caller-supplied key order for entries that exist in
// `order`. Entries not in order are appended at the end.
func sealFrontmatter(
	props map[string]any,
	order []string,
	propGroups map[string]string,
	dataKey []byte,
) (out map[string]any, newOrder []string, err error) {
	out = make(map[string]any, len(props)+1)
	newOrder = make([]string, 0, len(order)+1)

	emit := func(name string, v any) error {
		outKey, outVal, err := sealPropertyIfNeeded(name, v, propGroups, dataKey)
		if err != nil {
			return err
		}
		out[outKey] = outVal
		newOrder = append(newOrder, outKey)
		return nil
	}

	for _, name := range order {
		v, present := props[name]
		if !present {
			continue
		}
		if err := emit(name, v); err != nil {
			return nil, nil, err
		}
	}
	// Catch properties not mentioned in order (defensive).
	for name, v := range props {
		if containsString(newOrder, name) || containsString(newOrder, applyEncKey(name)) {
			continue
		}
		if err := emit(name, v); err != nil {
			return nil, nil, err
		}
	}
	return out, newOrder, nil
}

// sealPropertyIfNeeded returns the wire-format key and value for
// one property. Cleartext properties pass through unchanged;
// encrypted properties are sealed and renamed with the _enc_v1_
// prefix.
func sealPropertyIfNeeded(
	name string,
	v any,
	propGroups map[string]string,
	dataKey []byte,
) (outKey string, outVal any, err error) {
	if propGroups[name] == "" {
		return name, v, nil
	}
	sealed, err := sealOne(dataKey, v)
	if err != nil {
		return "", nil, &EncryptionError{
			Kind:     ErrKindCorruptedFile,
			Property: name,
			Cause:    err,
		}
	}
	return applyEncKey(name), sealed, nil
}

func sealBodyIfNeeded(
	rawBody string,
	dataKey []byte,
	encrypted bool,
	out map[string]any,
	newOrder []string,
) (sealedBody string, newOrderOut []string, err error) {
	if !encrypted {
		return rawBody, newOrder, nil
	}
	if rawBody == "" {
		return "", newOrder, nil
	}
	encBody, err := encryption.Seal([]byte(rawBody), dataKey)
	if err != nil {
		return "", nil, fmt.Errorf("fsstore: seal body: %w", err)
	}
	out[encryptedBodyKey] = base64.StdEncoding.EncodeToString(encBody)
	return "", append(newOrder, encryptedBodyKey), nil
}

// sealOne seals a single property value. Non-string values are
// marshaled to their YAML scalar form before sealing. Opaque values
// cannot appear here — sealProperties rejects them up front.
func sealOne(dataKey []byte, v any) (string, error) {
	raw := valueToBytes(v)
	sealed, err := encryption.Seal(raw, dataKey)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// valueToBytes converts a property value into the byte sequence that
// gets sealed. Strings pass through as UTF-8. Other scalar types are
// marshaled via fmt.Sprintf using %v (preserves round-trip for
// numbers, booleans, nil). This is intentionally conservative — if
// users need structured encrypted values, they can pre-marshal.
func valueToBytes(v any) []byte {
	switch x := v.(type) {
	case string:
		return []byte(x)
	case []byte:
		return x
	case nil:
		return nil
	default:
		return fmt.Appendf(nil, "%v", x)
	}
}

// containsString reports whether a string appears in a slice.
func containsString(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// unsealProperties is the inverse of sealProperties.
//
// It walks the just-parsed frontmatter map, identifies _enc_v1_*
// entries, looks up the matching group wraps in the _encryption
// block, unwraps the data key via Crypto.UnwrapAny, and decrypts each
// value. Decrypted properties land in the output map under their
// plain (non-prefixed) name.
//
// If a property's group has no usable private key locally (partial
// decrypt), the property surfaces as an encryption.Opaque holding the
// raw wire-format bytes. Other properties load unchanged.
//
// Body handling: if _encrypted_body is present, its ciphertext is
// decrypted (or returned as Opaque) and the original body content is
// replaced with the decrypted plaintext. The returned body string
// always reflects in-memory cleartext (or the empty string when the
// body couldn't be decrypted — fsstore surfaces that as part of
// properties-level Opaque handling in Step 3).
//
// Crypto == nil: if any _enc_v1_* key or _encryption block is
// present, returns ErrKindMissingKeyring. Otherwise returns the
// input unchanged.
func unsealProperties(
	crypto Crypto,
	fm map[string]any,
	rawBody string,
) (out map[string]any, body string, opaqueProps map[string]bool, err error) {
	opaqueProps = make(map[string]bool)
	if !hasEncryptionMarkers(fm) {
		return fm, rawBody, opaqueProps, nil
	}
	if crypto == nil || !crypto.HasPrivateKey() {
		return nil, "", nil, &EncryptionError{Kind: ErrKindMissingKeyring}
	}

	dataKeys, err := unsealEnvelope(crypto, fm)
	if err != nil {
		return nil, "", nil, err
	}

	out, err = unsealFrontmatter(fm, dataKeys, opaqueProps)
	if err != nil {
		return nil, "", nil, err
	}

	body, err = unsealBody(fm, rawBody, dataKeys)
	if err != nil {
		return nil, "", nil, err
	}
	return out, body, opaqueProps, nil
}

// hasEncryptionMarkers returns true if the frontmatter contains any
// key that indicates encryption was applied — either an _enc_v1_* key,
// the _encryption envelope block, or an _encrypted_body entry.
func hasEncryptionMarkers(fm map[string]any) bool {
	for k := range fm {
		if _, ok := stripEncKey(k); ok {
			return true
		}
		if k == encryptionKey || k == encryptedBodyKey {
			return true
		}
	}
	return false
}

// unsealEnvelope parses the _encryption block and unwraps one data
// key per group that matches our local keyring. Groups whose wrap
// map has no matching key are silently skipped (partial decrypt).
func unsealEnvelope(crypto Crypto, fm map[string]any) (map[string][]byte, error) {
	envRaw, ok := fm[encryptionKey]
	if !ok {
		return nil, &EncryptionError{
			Kind:  ErrKindCorruptedFile,
			Cause: errors.New("_enc_v1_* key without _encryption block"),
		}
	}
	envelope, err := parseEncryptionBlock(envRaw)
	if err != nil {
		return nil, &EncryptionError{Kind: ErrKindCorruptedFile, Cause: err}
	}

	dataKeys := make(map[string][]byte, len(envelope.dataKeys))
	for group, wraps := range envelope.dataKeys {
		dk, _, uwErr := crypto.UnwrapAny(wraps)
		if uwErr == nil {
			dataKeys[group] = dk
			continue
		}
		if errors.Is(uwErr, encryption.ErrNoMatchingKey) {
			continue // partial decrypt
		}
		return nil, classifyUnwrapError(uwErr)
	}
	return dataKeys, nil
}

func classifyUnwrapError(err error) error {
	switch {
	case errors.Is(err, encryption.ErrDecrypt), errors.Is(err, encryption.ErrBadBlob):
		return &EncryptionError{Kind: ErrKindCorruptedFile, Cause: err}
	case errors.Is(err, encryption.ErrNoPrivateKey):
		return &EncryptionError{Kind: ErrKindMissingKeyring, Cause: err}
	}
	return err
}

// unsealFrontmatter walks the frontmatter and decrypts _enc_v1_*
// entries, storing results under plain (non-prefixed) names. Values
// that can't be decrypted surface as encryption.Opaque.
func unsealFrontmatter(
	fm map[string]any,
	dataKeys map[string][]byte,
	opaqueProps map[string]bool,
) (map[string]any, error) {
	out := make(map[string]any, len(fm))
	for k, v := range fm {
		if k == encryptionKey || k == encryptedBodyKey {
			continue
		}
		plain, prefixed := stripEncKey(k)
		if !prefixed {
			out[k] = v
			continue
		}
		if err := unsealOneProperty(plain, v, dataKeys, out, opaqueProps); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func unsealOneProperty(
	plain string,
	v any,
	dataKeys map[string][]byte,
	out map[string]any,
	opaqueProps map[string]bool,
) error {
	cipher, err := decodeCiphertext(plain, v)
	if err != nil {
		return err
	}
	if decrypted, ok := tryDecryptWithAny(cipher, dataKeys); ok {
		out[plain] = string(decrypted)
		return nil
	}
	out[plain] = encryption.NewOpaque(cipher)
	opaqueProps[plain] = true
	return nil
}

func decodeCiphertext(prop string, v any) ([]byte, error) {
	b64, ok := v.(string)
	if !ok {
		return nil, &EncryptionError{
			Kind:     ErrKindCorruptedFile,
			Property: prop,
			Cause:    fmt.Errorf("_enc_v1_%s value must be string, got %T", prop, v),
		}
	}
	cipher, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, &EncryptionError{
			Kind:     ErrKindCorruptedFile,
			Property: prop,
			Cause:    err,
		}
	}
	return cipher, nil
}

// unsealBody decrypts an _encrypted_body entry if present. A body
// that can't be decrypted surfaces as the empty string (callers
// decide how to present that in-memory).
func unsealBody(fm map[string]any, rawBody string, dataKeys map[string][]byte) (string, error) {
	encRaw, ok := fm[encryptedBodyKey]
	if !ok {
		return rawBody, nil
	}
	b64, ok := encRaw.(string)
	if !ok {
		return "", &EncryptionError{
			Kind:  ErrKindCorruptedFile,
			Cause: fmt.Errorf("_encrypted_body must be string, got %T", encRaw),
		}
	}
	cipher, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", &EncryptionError{Kind: ErrKindCorruptedFile, Cause: err}
	}
	if plain, ok := tryDecryptWithAny(cipher, dataKeys); ok {
		return string(plain), nil
	}
	return "", nil
}

// tryDecryptWithAny iterates data keys and returns the first
// successful decryption. Useful when a property's group is not
// directly indicated on disk.
func tryDecryptWithAny(cipher []byte, keys map[string][]byte) ([]byte, bool) {
	// Sort group names for deterministic iteration (helps with test
	// reproducibility when multiple groups are in play).
	names := make([]string, 0, len(keys))
	for g := range keys {
		names = append(names, g)
	}
	sort.Strings(names)
	for _, g := range names {
		plain, err := encryption.Open(cipher, keys[g])
		if err == nil {
			return plain, true
		}
	}
	return nil, false
}
