package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/feed"
	"github.com/Sourcehaven-BV/rela/internal/ingest"
	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// Deps are the collaborators of one poll.
//
// Ingest carries the write path. Its reader and writer decide what the
// subscription may see and change, through the principal on the ctx, which
// the caller sets to system:subscription:<name>.
type Deps struct {
	Ingest ingest.Deps
	State  state.KV
	// Secrets returns the project's global secrets. Called only when the
	// subscription declares auth.
	Secrets func() (map[string]string, error)
	Fetcher *Fetcher
	Logger  *slog.Logger
	// Now is the run time behind {{now}} and {{today}}. Nil: accepted,
	// meaning time.Now.
	Now func() time.Time
}

func (d Deps) validate() error {
	switch {
	case d.State == nil:
		return errors.New("subscription: no state store")
	case d.Secrets == nil:
		return errors.New("subscription: no secrets source")
	case d.Fetcher == nil:
		return errors.New("subscription: no fetcher")
	case d.Logger == nil:
		return errors.New("subscription: no logger")
	}
	return nil
}

// Report counts what one poll did.
type Report struct {
	NotModified bool
	Items       int // items in the feed
	Dropped     int // older items past max_items
	Created     int
	Updated     int
	Unchanged   int // seen before with the same content, or found while rebaselining
	Skipped     int // no identity
	Rejected    int // refused by validation; recorded as seen
	Conflicted  int // kept colliding on a unique value; tried again on the next full fetch
}

// Run polls one subscription and imports its new and changed items.
//
// A transient failure (fetch, parse, store, conflicts, ctx) stops the run and
// is returned, so the scheduler retries it; the validators are then left as
// they were, and the next poll fetches the full feed again. Items handled
// before the failure are still recorded as seen. A permanent rejection of one
// item is logged and recorded, so a single bad item cannot fail every poll.
func Run(ctx context.Context, name string, sub Subscription, d Deps) (Report, error) {
	if err := d.validate(); err != nil {
		return Report{}, err
	}
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	config, err := configHash(sub)
	if err != nil {
		return Report{}, err
	}
	rec, err := loadRecord(ctx, d.State, name, config)
	if err != nil {
		return Report{}, err
	}

	target, err := url.Parse(sub.URL)
	if err != nil {
		return Report{}, errors.New("subscription: url does not parse")
	}
	header, err := authHeader(sub.Auth, d.Secrets)
	if err != nil {
		return Report{}, err
	}
	resp, err := d.Fetcher.fetch(ctx, request{
		url: target, header: header, etag: rec.ETag, lastModified: rec.LastModified,
	})
	if err != nil {
		return Report{}, err
	}
	if resp.notModified {
		return Report{NotModified: true}, nil
	}
	parsed, err := feed.Parse(resp.body)
	if err != nil {
		return Report{}, fmt.Errorf("parse feed from %s: %w", originOf(target), err)
	}

	items, dropped := newest(parsed.Items, sub.EffectiveMaxItems())
	report := Report{Items: len(parsed.Items), Dropped: dropped}

	mapping := sub.Mapping
	if rec.Rebaseline {
		mapping.Then = nil
	}
	seen := newSeenSet(rec.Seen)
	runErr := importItems(ctx, name, mapping, d, &parsed, items, resp.finalURL, now(), seen, &report)

	rec.Seen = seen.entries(seenFactor * sub.EffectiveMaxItems())
	if runErr == nil {
		rec.Rebaseline = false
		// A conflicted item is not marked seen, but a 304 would hide it
		// until the publisher changes the feed. Without validators the
		// next poll fetches the feed and tries it again.
		rec.ETag, rec.LastModified = "", ""
		if report.Conflicted == 0 {
			rec.ETag, rec.LastModified = resp.etag, resp.lastModified
		}
	}
	if err := saveRecord(ctx, d.State, name, rec); err != nil {
		return report, errors.Join(runErr, err)
	}
	return report, runErr
}

// importItems runs the mapping for each item, oldest first, and stops at the
// first transient failure.
func importItems(
	ctx context.Context, name string, mapping ingestspec.Mapping, d Deps, parsed *feed.Feed, items []feed.Item,
	base *url.URL, now time.Time, seen *seenSet, report *Report,
) error {
	for _, it := range items {
		src := &itemSource{name: name, feed: parsed, item: it, base: base, now: now, warn: d.Logger.Warn}
		values := ingest.MatchValues(mapping, src)
		if slices.Contains(mapValues(values), "") {
			report.Skipped++
			d.Logger.Warn("subscription item has no identity; skipped",
				"subscription", name, "match", mapping.Find.Match)
			continue
		}
		id, content := identityHash(mapping.Find.Match, values), contentHash(parsed, it)
		if seen.unchanged(id, content) {
			report.Unchanged++
			seen.touch(id)
			continue
		}

		res, err := ingest.Run(ctx, d.Ingest, mapping, src)
		switch {
		case err == nil:
		case errors.Is(err, ingest.ErrConflictExhausted):
			// A unique collision that outlived the retries is most likely
			// another entity holding a unique value this item maps to,
			// which no retry fixes. It is not marked seen, so a later 200
			// tries the item again, but it does not hold up the feed.
			report.Conflicted++
			d.Logger.Warn("subscription item kept conflicting", "subscription", name, "error", err)
			continue
		case permanent(err):
			report.Rejected++
			seen.mark(id, content)
			d.Logger.Warn("subscription item rejected", "subscription", name, "error", err)
			continue
		default:
			return fmt.Errorf("import item: %w", err)
		}
		seen.mark(id, content)
		switch res.Action {
		case ingest.ActionCreated:
			report.Created++
		case ingest.ActionUpdated:
			if len(mapping.Then) == 0 {
				report.Unchanged++ // found, and nothing to apply
			} else {
				report.Updated++
			}
		}
	}
	return nil
}

// permanent reports whether err rejects the item itself, so that retrying
// the same item would fail the same way. Anything else may pass on a later
// poll and is left to the scheduler's retry.
func permanent(err error) bool {
	if _, ok := errors.AsType[*entitymanager.ValidationError](err); ok {
		return !entitymanager.IsUniqueViolation(err)
	}
	if _, ok := errors.AsType[*entitymanager.ComputedWriteError](err); ok {
		return true
	}
	return false
}

// newest returns the max newest items, oldest first, and how many it dropped.
//
// Feeds usually list newest first, but RSS does not require it, so items are
// ordered by date. An item without a date sorts after dated ones, keeping its
// place in the document among its peers.
func newest(items []feed.Item, limit int) (kept []feed.Item, dropped int) {
	kept = slices.Clone(items)
	slices.SortStableFunc(kept, func(a, b feed.Item) int {
		return itemTime(b).Compare(itemTime(a))
	})
	if len(kept) > limit {
		dropped = len(kept) - limit
		kept = kept[:limit]
	}
	slices.Reverse(kept)
	return kept, dropped
}

func itemTime(it feed.Item) time.Time {
	if !it.Published.IsZero() {
		return it.Published
	}
	return it.Updated
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// authHeader builds the credential header from the named secret. The error
// names the secret, never its value.
func authHeader(auth *Auth, load func() (map[string]string, error)) (http.Header, error) {
	h := http.Header{}
	if auth == nil {
		return h, nil
	}
	all, err := load()
	if err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return nil, fmt.Errorf("subscription auth: %w", err)
	}
	secret, ok := all[auth.Secret]
	if !ok || secret == "" {
		return nil, fmt.Errorf("subscription auth: secret %q is not set", auth.Secret)
	}
	switch auth.Type {
	case AuthBearer:
		h.Set("Authorization", "Bearer "+secret)
	case AuthBasic:
		h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth.Username+":"+secret)))
	case AuthHeader:
		h.Set(auth.Header, secret)
	default:
		return nil, fmt.Errorf("subscription auth: unknown type %q", auth.Type)
	}
	return h, nil
}
