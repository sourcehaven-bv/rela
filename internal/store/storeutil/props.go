package storeutil

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidateProperties is the store-boundary gate for property text. It
// rejects any property key or string value, at any nesting a property value
// can take, that is not valid UTF-8 or that contains a NUL byte
// (BUG-X7ICNM).
//
// The backends disagreed about such values, and the lax side was the unsafe
// one. fsstore refused them outright (yaml.v3 cannot marshal invalid UTF-8 as
// a text scalar) while pgstore and sqlitestore, which serialize properties
// with encoding/json, silently substituted U+FFFD for each bad byte and
// reported success — the write "succeeded" and the caller read back a value
// they never wrote. memstore kept the bytes verbatim, agreeing with neither.
//
// The rule lives here, called from every backend's entity and relation write
// path, rather than inside the backends that happened to be lax: one
// implementation, four call sites, the same arrangement as [ValidateID]. It
// is also the validity oracle the storetest fuzz targets enforce
// directionally — anything this rejects, every store MUST reject; anything it
// accepts must round-trip byte-for-byte.
//
// Rejecting rather than repairing is deliberate. Invalid UTF-8 has no
// representation in YAML text at all, so the only ways to "accept" it are a
// substitution the caller is not told about or an escape scheme for data
// nobody intends to store. The mirror-image bug (BUG-B1RA3J) went the other
// way — a legitimate value fsstore could not serialize, fixed by making
// fsstore store it — because there the value was representable. Here it is
// not, and a store that persists something other than what it was given has
// failed at its one job.
//
// NUL is rejected for the same reason from the other side: YAML and JSON can
// escape it, but Postgres cannot hold U+0000 in a text or jsonb column at
// all, so pgstore alone refused it (loudly, at least) while the other three
// stored it. The fuzz round-trip found that divergence the first time it
// ran. A NUL is never legitimate property text; better one rule every
// backend applies than a value whose validity depends on the deployment.
func ValidateProperties(props map[string]any) error {
	for key, val := range props {
		if err := checkText("property key "+strconv.Quote(key), key); err != nil {
			return err
		}
		if err := validatePropertyValue(key, val); err != nil {
			return err
		}
	}
	return nil
}

// checkText is the one rule, applied to every key and string value: valid
// UTF-8 and no NUL. what names the offending location in the error.
func checkText(what, s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("store: %s: invalid UTF-8", what)
	}
	if strings.IndexByte(s, 0) >= 0 {
		return fmt.Errorf("store: %s: contains NUL", what)
	}
	return nil
}

// validatePropertyValue walks the container shapes a property value can take:
// the ones yaml.v3, encoding/json and the importer produce. Anything that is
// not a string or a container of them (numbers, bools, nil, times) carries no
// text and cannot be invalid. A container type missing here is a hole, not a
// pass — [entity.CloneValue] and canonical.normalize walk the same domain and
// should agree with this switch.
func validatePropertyValue(path string, val any) error {
	switch v := val.(type) {
	case string:
		return checkText("property "+strconv.Quote(path), v)
	case []string:
		for i, s := range v {
			if err := checkText("property "+strconv.Quote(indexed(path, i)), s); err != nil {
				return err
			}
		}
	case []any:
		for i, item := range v {
			if err := validatePropertyValue(indexed(path, i), item); err != nil {
				return err
			}
		}
	case map[string]string:
		for k, s := range v {
			if err := checkKey(path, k); err != nil {
				return err
			}
			if err := checkText("property "+strconv.Quote(path+"."+k), s); err != nil {
				return err
			}
		}
	case map[string]any:
		for k, item := range v {
			if err := checkKey(path, k); err != nil {
				return err
			}
			if err := validatePropertyValue(path+"."+k, item); err != nil {
				return err
			}
		}
	case map[any]any:
		return validateAnyKeyedMap(path, v)
	}
	return nil
}

// validateAnyKeyedMap handles the shape yaml.v3 decodes a nested mapping with
// a non-string key into, which the importer hands to CreateEntity as-is. Only
// a string key can carry bad text; values are walked regardless.
func validateAnyKeyedMap(path string, m map[any]any) error {
	for k, item := range m {
		if ks, ok := k.(string); ok {
			if err := checkKey(path, ks); err != nil {
				return err
			}
		}
		if err := validatePropertyValue(path+"."+fmt.Sprint(k), item); err != nil {
			return err
		}
	}
	return nil
}

func checkKey(path, key string) error {
	return checkText(fmt.Sprintf("property %q: key %q", path, key), key)
}

func indexed(path string, i int) string {
	return fmt.Sprintf("%s[%d]", path, i)
}
