package dataentry_test

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestGoldenParity is the golden wire-parity corpus (TKT-0M4H0S). It drives
// the production router of an app built like rela-server's (see
// golden_fixture_test.go) and records, per case, the status, six response
// headers, the store reads and the body. The recordings in testdata/golden
// are the wire contract a refactor of the access layer must keep: a change in
// any of them is either the intended fix of a recorded gap or a regression.
//
// Cases named after a bug (BUG-2TPO2F, BUG-C8DHGF, BUG-VUNO11)
// record a known gap as it is today. The fix of that bug is expected to
// change those files.
//
// Rewrite the recordings with:
//
//	go test ./internal/dataentry -run TestGoldenParity -update
//
// Every case runs on its own app, in parallel with the others; the recordings
// are compared once all have run.
func TestGoldenParity(t *testing.T) {
	// A broken fixture fails every case the same way; build one app first so
	// it fails once.
	newGoldenApp(t, goldenDeclHeader)
	cases := goldenCases()
	results := make([]string, len(cases))
	names := make(map[string]bool, len(cases))
	t.Run("record", func(t *testing.T) {
		for i, c := range cases {
			name := c.name()
			if names[name] {
				t.Fatalf("two corpus cases are named %s", name)
			}
			names[name] = true
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				results[i] = recordGoldenCase(t, c)
			})
		}
	})

	// A file shared by several cases holds the first one's recording; the
	// others must match it.
	var files []string
	byFile := map[string][]int{}
	for i, c := range cases {
		file := c.file()
		if _, ok := byFile[file]; !ok {
			files = append(files, file)
		}
		byFile[file] = append(byFile[file], i)
	}
	for _, file := range files {
		idx := byFile[file]
		t.Run(file, func(t *testing.T) {
			if len(idx) > 1 && !cases[idx[0]].sharesFile() {
				t.Fatalf("two corpus cases record to %s", file)
			}
			first := results[idx[0]]
			if first == "" {
				t.Skipf("recording %s failed; see its case", cases[idx[0]].name())
			}
			checkGolden(t, filepath.Join(goldenDir, file), first)
			for _, i := range idx[1:] {
				if results[i] != "" && results[i] != first {
					t.Errorf("%s is recorded as the same for %s, but %s gets (- recorded, + now):\n%s",
						file, cases[i].sharedBy(), cases[i].name(), lineDiff(first, results[i]))
				}
			}
		})
	}
	seen := make(map[string]bool, len(byFile))
	for file := range byFile {
		seen[file] = true
	}
	t.Run("no stale recordings", func(t *testing.T) {
		checkNoStaleGoldens(t, seen)
	})
}

// goldenAnonymousSlug names the one recording every request without an
// identity shares in variant v. In header mode that answer is BUG-C8DHGF's
// 500 (an unstamped principal), whatever the path; over the JWT gate it is
// the refusal of a missing assertion. A fix that makes the answer depend on
// the path, such as a public read, has to give those requests their own
// recordings again.
func goldenAnonymousSlug(v goldenVariant) string {
	if v.jwt {
		return "anonymous"
	}
	return "BUG-C8DHGF-anonymous"
}

// goldenInstance matches the problem-details member that echoes the request
// path, which a shared anonymous recording cannot keep.
var goldenInstance = regexp.MustCompile(`"instance": "[^"]*"`)

// goldenUpdate rewrites the recordings instead of comparing against them.
var goldenUpdate = flag.Bool("update", false, "rewrite the golden wire-parity corpus in testdata/golden")

// goldenDir holds one recording per corpus case, in a directory per variant.
const goldenDir = "testdata/golden"

// The corpus variants. goldenDeclHeader is the full matrix; the others run the
// subset of cases whose answer depends on the kind of ACL or on the
// identity source.
var (
	goldenDeclHeader = goldenVariant{name: "decl-header", mode: goldenDeclarative}
	goldenDeclJWT    = goldenVariant{name: "decl-jwt", mode: goldenDeclarative, jwt: true}
	goldenNopVariant = goldenVariant{name: "nop", mode: goldenNop}
	goldenROVariant  = goldenVariant{name: "readonly", mode: goldenReadOnly}
)

// goldenCase is one recorded request.
type goldenCase struct {
	goldenEndpoint
	variant goldenVariant
	// as is the login the request carries; empty sends no identity.
	as string
	// assertion, when set, replaces the signed assertion a JWT variant
	// would send for as.
	assertion string
	// shared says the case records to the file its endpoint shares across
	// the authenticated principals of its variant.
	shared bool
}

// anonymous reports whether c carries no identity at all.
func (c goldenCase) anonymous() bool { return c.as == "" && c.assertion == "" }

// sharesFile reports whether c records to a file other cases record to too.
func (c goldenCase) sharesFile() bool { return c.shared || c.anonymous() }

// sharedBy says which cases share c's file, for a mismatch message.
func (c goldenCase) sharedBy() string {
	if c.anonymous() {
		return "every request without an identity"
	}
	return "every authenticated principal"
}

// name is c's subtest name: its file, plus what tells it apart from the
// other cases sharing that file.
func (c goldenCase) name() string {
	switch {
	case c.anonymous():
		return c.file() + " for " + c.slug
	case c.shared:
		return c.file() + " as " + c.who()
	default:
		return c.file()
	}
}

// who names the principal of c in file and subtest names.
func (c goldenCase) who() string {
	switch {
	case c.assertion != "":
		return "forged"
	case c.as == "":
		return "anonymous"
	default:
		return strings.TrimSuffix(c.as, "@example.test")
	}
}

// file is the recording path of c, relative to goldenDir. Every request
// without an identity shares one file per variant (see goldenAnonymousSlug).
func (c goldenCase) file() string {
	if c.anonymous() {
		return filepath.Join(c.variant.name, goldenAnonymousSlug(c.variant)+".golden")
	}
	who := c.who()
	if c.shared {
		who = "authenticated"
	}
	return filepath.Join(c.variant.name, c.slug+"."+who+".golden")
}

// goldenEndpoint is a request the corpus sends under several principals.
type goldenEndpoint struct {
	// slug names the request. A slug that starts with a bug id records the
	// gap that bug describes.
	slug         string
	method, path string
	body         string
	// contentType overrides the type a body is sent with (JSON by default).
	contentType string
	// accept, when set, is sent as the Accept header.
	accept string
	// multipart sends body as the file part of a multipart upload.
	multipart bool
	// formFile names the multipart file part as "field/filename"
	// (default "file/new.txt").
	formFile string
	// minted matches values the request draws from a random source the
	// corpus cannot fix: comment ids, attachment path segments and script
	// error correlation ids all come from crypto/rand with no seam. The
	// recording replaces each match with a placeholder.
	minted *regexp.Regexp
	// unorderedArrays names the top-level arrays of the JSON body that the
	// handler builds in map order. The recording sorts each one's elements,
	// objects by their "name" member and strings by value, and leaves the
	// rest of the body as sent.
	unorderedArrays []string
	// principalFree marks an endpoint whose answer does not depend on who
	// asks, once the request is authenticated: operator configuration and
	// static descriptions. Its authenticated principals share one recording.
	principalFree bool
}

// Patterns for goldenEndpoint.minted.
var (
	mintedCommentID     = regexp.MustCompile(`"id": "[a-z2-7]{16}"`)
	mintedAttachmentKey = regexp.MustCompile(`/[0-9a-f]{16}/`)
	mintedCorrelationID = regexp.MustCompile(`"correlation_id": "[0-9a-f]+"`)
	mintedPileID        = regexp.MustCompile(`PIL-[0-9A-Z]+`)
	// A lease's id, its fencing token (a clock reading) and its expiry.
	mintedLease = regexp.MustCompile(`[0-9a-f]{32}|\b\d{13,}\b|\d{4}-\d\d-\d\dT[0-9:.]+(Z|[+-]\d\d:\d\d)`)
)

// goldenCalDAVQuery asks a collection for every to-do with its data.
const goldenCalDAVQuery = `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VTODO"/></C:comp-filter></C:filter>
</C:calendar-query>`

// goldenReads are the read endpoints. Each runs for every principal of the
// header-mode declarative variant.
//
// The fixture's hidden values: TKT-4..6 are hidden rows for the viewer;
// on the rows it reads, budget is always hidden, secret and due when the
// ticket is closed (TKT-2), and note unless the ticket contains a closed one.
var goldenReads = []goldenEndpoint{
	{slug: "help", method: http.MethodGet, path: "/api/help/ticket", principalFree: true},
	// Not principal-free: each world says whether the caller may read it.
	{slug: "schema", method: http.MethodGet, path: "/api/v1/_schema"},
	{slug: "schema-type", method: http.MethodGet, path: "/api/v1/_schema/types/ticket", principalFree: true},
	{slug: "schema-types", method: http.MethodGet, path: "/api/v1/_schema/types", principalFree: true},
	{slug: "schema-relations", method: http.MethodGet, path: "/api/v1/_schema/relations", principalFree: true},
	{slug: "schema-channels", method: http.MethodGet, path: "/api/v1/_schema/channels", principalFree: true},
	{slug: "config", method: http.MethodGet, path: "/api/v1/_config", principalFree: true},
	{slug: "configure", method: http.MethodGet, path: "/api/v1/_configure", principalFree: true},
	{slug: "configure-key", method: http.MethodGet, path: "/api/v1/_configure/acl", principalFree: true},
	{slug: "openapi", method: http.MethodGet, path: "/api/v1/_openapi.json", principalFree: true},
	{slug: "me", method: http.MethodGet, path: "/api/v1/_me"},
	{slug: "feed", method: http.MethodGet, path: "/api/v1/_feeds/due.ics"},
	{slug: "search-title", method: http.MethodGet, path: "/api/v1/_search?q=Alpha"},
	{slug: "search-secret", method: http.MethodGet, path: "/api/v1/_search?q=delta-secret"},
	// bravo-secret is hidden on a row the viewer reads.
	{slug: "search-secret-visible-row", method: http.MethodGet, path: "/api/v1/_search?q=bravo-secret"},
	{slug: "position", method: http.MethodGet, path: "/api/v1/_position?id=TKT-3&scope=" +
		url.QueryEscape(`{"source":"list","type":"ticket"}`)},
	{slug: "analyze", method: http.MethodGet, path: "/api/v1/_analyze"},
	{slug: "next-action", method: http.MethodGet, path: "/api/v1/_next_action"},
	// All three arrays are built in map order; BUG-E0CZZL tracks it.
	{slug: "settings", method: http.MethodGet, path: "/api/v1/_settings",
		unorderedArrays: []string{"allProperties", "allRelations", "entityTypes"}},
	{slug: "palette", method: http.MethodGet, path: "/api/v1/_palette", principalFree: true},
	{slug: "theme-logo", method: http.MethodGet, path: "/api/v1/_theme/logo", principalFree: true},
	{slug: "theme-export", method: http.MethodGet, path: "/api/v1/_theme/export", principalFree: true},
	{slug: "sidepanel", method: http.MethodGet, path: "/api/v1/_sidepanel/ticket_form/TKT-1"},
	{slug: "sidebar", method: http.MethodGet, path: "/api/v1/_sidebar"},
	{slug: "nav-status", method: http.MethodGet, path: "/api/v1/_nav_status"},
	{slug: "nav-items", method: http.MethodGet, path: "/api/v1/_nav_items"},
	{slug: "dashboard", method: http.MethodGet, path: "/api/v1/_dashboard"},
	// The conflict routes need git:sync, which only admin holds. The fixture
	// holds a conflicted copy of TKT-4, which the viewer may not read; nobody
	// without git:sync gets its path or values (BUG-T6HNKS).
	{slug: "conflicts", method: http.MethodGet, path: "/api/v1/_conflicts"},
	{slug: "conflict", method: http.MethodGet, path: "/api/v1/_conflicts/TKT-1"},
	{slug: "conflict-hidden", method: http.MethodGet, path: "/api/v1/_conflicts/entities/ticket/TKT-4.md"},
	{slug: "document", method: http.MethodGet, path: "/api/v1/_documents/report"},
	{slug: "document-elevated", method: http.MethodGet, path: "/api/v1/_documents/elevated_report"},
	{slug: "export-document", method: http.MethodGet, path: "/api/v1/_documents/report/_export?transform=copy"},
	{slug: "history", method: http.MethodGet, path: "/api/v1/_history/ticket/TKT-1"},
	{slug: "history-version", method: http.MethodGet, path: "/api/v1/_history/ticket/TKT-1/1"},
	{slug: "history-hidden", method: http.MethodGet, path: "/api/v1/_history/ticket/TKT-4"},
	{slug: "history-absent", method: http.MethodGet, path: "/api/v1/_history/ticket/TKT-99"},
	{slug: "relation-history", method: http.MethodGet,
		path: "/api/v1/_relation_history/ticket/TKT-1/implements/POL-1"},
	{slug: "relation-history-version", method: http.MethodGet,
		path: "/api/v1/_relation_history/ticket/TKT-1/implements/POL-1/2"},
	{slug: "commands", method: http.MethodGet, path: "/api/v1/_commands"},
	{slug: "transforms", method: http.MethodGet, path: "/api/v1/_transforms", principalFree: true},
	{slug: "comments", method: http.MethodGet, path: "/api/v1/_comments/ticket/TKT-1"},
	{slug: "comments-hidden", method: http.MethodGet, path: "/api/v1/_comments/ticket/TKT-4"},
	{slug: "comments-absent", method: http.MethodGet, path: "/api/v1/_comments/ticket/TKT-99"},
	{slug: "piles", method: http.MethodGet, path: "/api/v1/_piles"},
	{slug: "pile-viewer", method: http.MethodGet, path: "/api/v1/_piles/PIL-GOLDC"},
	{slug: "leases", method: http.MethodGet, path: "/api/v1/_leases"},
	{slug: "presence", method: http.MethodGet, path: "/api/v1/tickets/TKT-1/_presence"},
	{slug: "presence-hidden", method: http.MethodGet, path: "/api/v1/tickets/TKT-4/_presence"},
	{slug: "presence-absent", method: http.MethodGet, path: "/api/v1/tickets/TKT-99/_presence"},
	// The viewer's pile holds TKT-5, which the viewer may not read.
	{slug: "pile-export", method: http.MethodGet, path: "/api/v1/_piles/PIL-GOLDC/_export?transform=copy"},
	{slug: "template", method: http.MethodGet, path: "/api/v1/_templates/ticket"},
	{slug: "view", method: http.MethodGet, path: "/api/v1/_views/ticket/TKT-1"},
	{slug: "view-closed", method: http.MethodGet, path: "/api/v1/_views/ticket/TKT-2"},
	{slug: "gantt", method: http.MethodGet, path: "/api/v1/_gantts/plan"},
	{slug: "app", method: http.MethodGet, path: "/api/v1/_apps/none/", principalFree: true},
	{slug: "caldav-report", method: "REPORT", path: "/api/v1/_caldav/principal/calendars/tickets/",
		body: goldenCalDAVQuery, contentType: "application/xml"},
	{slug: "caldav-get", method: http.MethodGet,
		path: "/api/v1/_caldav/principal/calendars/tickets/ticket--TKT-1@rela.ics"},
	{slug: "caldav-get-hidden", method: http.MethodGet,
		path: "/api/v1/_caldav/principal/calendars/tickets/ticket--TKT-4@rela.ics"},
	{slug: "list", method: http.MethodGet, path: "/api/v1/tickets/"},
	{slug: "list-filtered", method: http.MethodGet, path: "/api/v1/tickets/?filter[status]=open&sort=-title"},
	{slug: "list-filter-hidden", method: http.MethodGet,
		path: "/api/v1/tickets/?filter[secret][contains]=brav"},
	{slug: "list-filter-hidden-eq", method: http.MethodGet,
		path: "/api/v1/tickets/?filter[secret]=bravo-secret"},
	{slug: "list-sort-hidden", method: http.MethodGet, path: "/api/v1/tickets/?sort=secret"},
	{slug: "list-include", method: http.MethodGet, path: "/api/v1/tickets/?include=*"},
	{slug: "list-policies", method: http.MethodGet, path: "/api/v1/policies/"},
	// The viewer is granted world:published and not world:editorial.
	{slug: "list-world-editorial", method: http.MethodGet, path: "/api/v1/policies/?world=editorial"},
	{slug: "list-world-published", method: http.MethodGet, path: "/api/v1/policies/?world=published"},
	{slug: "export-list", method: http.MethodGet, path: "/api/v1/tickets/_export?transform=copy"},
	{slug: "item", method: http.MethodGet, path: "/api/v1/tickets/TKT-1"},
	{slug: "item-closed", method: http.MethodGet, path: "/api/v1/tickets/TKT-2"},
	{slug: "item-hidden", method: http.MethodGet, path: "/api/v1/tickets/TKT-4"},
	{slug: "item-absent", method: http.MethodGet, path: "/api/v1/tickets/TKT-99"},
	{slug: "item-policy", method: http.MethodGet, path: "/api/v1/policies/POL-1"},
	{slug: "item-policy-draft", method: http.MethodGet, path: "/api/v1/policies/POL-1@draft"},
	{slug: "export-item", method: http.MethodGet, path: "/api/v1/tickets/TKT-2/_export?transform=copy"},
	{slug: "relations", method: http.MethodGet, path: "/api/v1/tickets/TKT-1/relations"},
	{slug: "relations-type", method: http.MethodGet, path: "/api/v1/tickets/TKT-3/relations/implements"},
	{slug: "relation-target", method: http.MethodGet, path: "/api/v1/tickets/TKT-1/relations/implements/POL-1"},
	{slug: "mentioned-by", method: http.MethodGet, path: "/api/v1/tickets/TKT-1/mentioned-by"},
	{slug: "attachment-file", method: http.MethodGet, path: "/api/v1/tickets/TKT-1/_attachments/screenshot/shot.txt"},
	{slug: "command-entity", method: http.MethodPost, path: "/api/command/entity_ids?entity_id=TKT-1"},
	{slug: "command-entity-closed", method: http.MethodPost, path: "/api/command/entity_ids?entity_id=TKT-2"},
	// The list-context command feeds the script raw rows: hidden tickets and
	// hidden fields reach its stdin, and the script echoes them.
	{slug: "BUG-2TPO2F-command-list-input", method: http.MethodPost, path: "/api/command/dump_ids?list_id=tickets"},
	{slug: "command-cancel", method: http.MethodPost, path: "/api/command-cancel/none", principalFree: true},
	{slug: "command-file", method: http.MethodGet, path: "/api/command-file/none", principalFree: true},
}

// goldenWrites are the write endpoints. Each case gets a fresh app, as
// every case does, so no write sees another's effect.
//
// The editor's grants: it may not write note, may not set status to
// archived, may add but not remove an implements edge, and lacks the
// close-ticket guard of open → closed.
var goldenWrites = []goldenEndpoint{
	// The create response serializes the new entity's raw outgoing edges, so
	// a caller who may not read POL-2 sees the edge an automation adds to it.
	{slug: "create", method: http.MethodPost, path: "/api/v1/tickets/",
		body: `{"properties":{"title":"Golf","status":"open","secret":"golf-secret"},` +
			`"relations":{"implements":{"data":[{"type":"policy","id":"POL-1"}]}}}`},
	{slug: "update", method: http.MethodPatch, path: "/api/v1/tickets/TKT-3",
		body: `{"properties":{"title":"Charlie renamed"}}`},
	{slug: "update-denied-field", method: http.MethodPatch, path: "/api/v1/tickets/TKT-3",
		body: `{"properties":{"note":"edited"}}`},
	{slug: "update-denied-option", method: http.MethodPatch, path: "/api/v1/tickets/TKT-2",
		body: `{"properties":{"status":"archived"}}`},
	{slug: "update-transition-guarded", method: http.MethodPatch, path: "/api/v1/tickets/TKT-3",
		body: `{"properties":{"status":"closed"}}`},
	{slug: "update-hidden", method: http.MethodPatch, path: "/api/v1/tickets/TKT-4",
		body: `{"properties":{"title":"Delta renamed"}}`},
	// POL-1 has two faces and the request names none, so a write must not
	// pick one.
	{slug: "update-bare-id", method: http.MethodPatch, path: "/api/v1/policies/POL-1",
		body: `{"properties":{"title":"Renamed"}}`},
	{slug: "relation-create", method: http.MethodPost, path: "/api/v1/tickets/TKT-2/relations/implements",
		body: `{"id":"POL-1"}`},
	// relation_grants lets the viewer, which may not update tickets, link
	// two tickets it reads.
	{slug: "relation-create-granted", method: http.MethodPost, path: "/api/v1/tickets/TKT-3/relations/contains",
		body: `{"id":"TKT-1"}`},
	{slug: "relation-delete-protected", method: http.MethodDelete,
		path: "/api/v1/tickets/TKT-1/relations/implements/POL-1"},
	{slug: "delete", method: http.MethodDelete, path: "/api/v1/tickets/TKT-6"},
	{slug: "entity-restore", method: http.MethodPost, path: "/api/v1/tickets/TKT-7/restore"},
	{slug: "entity-clone", method: http.MethodPost, path: "/api/v1/tickets/TKT-1/_actions/clone"},
	// The action sets status open → closed, so the transition guard applies.
	{slug: "action-transition", method: http.MethodPost, path: "/api/v1/_action/close",
		body: `{"entity_id":"TKT-3"}`, minted: mintedCorrelationID},
	// The upload response serializes TKT-3's raw outgoing edges, including
	// the one to the draft-only POL-2.
	{slug: "upload", method: http.MethodPost, path: "/api/v1/tickets/TKT-3/_attachments/screenshot",
		body: "new shot", multipart: true, minted: mintedAttachmentKey},
	// The restore response serializes TKT-3's raw outgoing edges too.
	{slug: "restore", method: http.MethodPost, path: "/api/v1/_history/ticket/TKT-3/1/restore"},
	{slug: "copy-publish", method: http.MethodPost, path: "/api/v1/_copies/publish", body: `{"source_id":"POL-2"}`},
	{slug: "comment-add", method: http.MethodPost, path: "/api/v1/_comments/ticket/TKT-1",
		body:   `{"anchor":{"kind":"property","ref":"status"},"body":"looks wrong"}`,
		minted: mintedCommentID},
	{slug: "comment-accept", method: http.MethodPost, path: "/api/v1/_comments/ticket/TKT-1/c-visible/accept"},
	{slug: "theme-import", method: http.MethodPost, path: "/api/v1/_theme/import", body: `{}`},
	// The logo upload takes PUT and POST alike.
	{slug: "theme-logo-put", method: http.MethodPut, path: "/api/v1/_theme/logo",
		body: goldenLogo, multipart: true, formFile: "logo/logo.svg"},
	{slug: "theme-logo-post", method: http.MethodPost, path: "/api/v1/_theme/logo",
		body: goldenLogo, multipart: true, formFile: "logo/logo.svg"},
	{slug: "theme-logo-delete", method: http.MethodDelete, path: "/api/v1/_theme/logo"},
	{slug: "next-action-feedback", method: http.MethodPost, path: "/api/v1/_next_action",
		body: `{"source":"none"}`},
	{slug: "relation-update", method: http.MethodPatch,
		path: "/api/v1/tickets/TKT-1/relations/implements/POL-1", body: `{"meta":{}}`},
	{slug: "attachment-delete", method: http.MethodDelete,
		path: "/api/v1/tickets/TKT-1/_attachments/screenshot/shot.txt"},
	// The upload takes PUT and POST alike.
	{slug: "upload-put", method: http.MethodPut, path: "/api/v1/tickets/TKT-3/_attachments/screenshot",
		body: "new shot", multipart: true, minted: mintedAttachmentKey},
	{slug: "caldav-put", method: http.MethodPut,
		path: "/api/v1/_caldav/principal/calendars/tickets/ticket--TKT-3@rela.ics",
		body: goldenCalDAVTodo("TKT-3", "Charlie renamed"), contentType: "text/calendar"},
	{slug: "caldav-put-hidden", method: http.MethodPut,
		path: "/api/v1/_caldav/principal/calendars/tickets/ticket--TKT-4@rela.ics",
		body: goldenCalDAVTodo("TKT-4", "Delta renamed"), contentType: "text/calendar"},
	{slug: "caldav-delete", method: http.MethodDelete,
		path: "/api/v1/_caldav/principal/calendars/tickets/ticket--TKT-6@rela.ics"},
	// The settings and palette writes take PUT and POST alike.
	{slug: "settings-put", method: http.MethodPut, path: "/api/v1/_settings", body: `{}`},
	{slug: "settings-post", method: http.MethodPost, path: "/api/v1/_settings", body: `{}`},
	{slug: "palette-put", method: http.MethodPut, path: "/api/v1/_palette", body: `{}`},
	{slug: "palette-post", method: http.MethodPost, path: "/api/v1/_palette", body: `{}`},
	{slug: "comment-update", method: http.MethodPatch, path: "/api/v1/_comments/ticket/TKT-1/c-visible",
		body: `{"body":"edited"}`},
	{slug: "comment-delete", method: http.MethodDelete, path: "/api/v1/_comments/ticket/TKT-1/c-visible"},
	// The pile writes name the viewer's pile, which only the viewer owns.
	{slug: "pile-create", method: http.MethodPost, path: "/api/v1/_piles",
		body: `{"name":"New","items":["TKT-1","TKT-4"]}`, minted: mintedPileID},
	{slug: "pile-update", method: http.MethodPatch, path: "/api/v1/_piles/PIL-GOLDC", body: `{"name":"Renamed"}`},
	{slug: "pile-delete", method: http.MethodDelete, path: "/api/v1/_piles/PIL-GOLDC"},
	{slug: "pile-add-items", method: http.MethodPost, path: "/api/v1/_piles/PIL-GOLDC/items",
		body: `{"items":["TKT-3","TKT-4"]}`},
	{slug: "pile-remove-items", method: http.MethodPost, path: "/api/v1/_piles/PIL-GOLDC/items/_remove",
		body: `{"items":["TKT-5"]}`},
	{slug: "lease-claim", method: http.MethodPost, path: "/api/v1/tickets/TKT-1/_lease",
		body: `{"label":"editing"}`, minted: mintedLease},
	{slug: "lease-claim-hidden", method: http.MethodPost, path: "/api/v1/tickets/TKT-4/_lease",
		body: `{"label":"editing"}`, minted: mintedLease},
	// A well-formed lease id that no claim returned.
	{slug: "lease-renew", method: http.MethodPost, path: "/api/v1/_leases/renew",
		body: `{"lease_id":"0123456789abcdef0123456789abcdef"}`},
	{slug: "lease-release", method: http.MethodPost, path: "/api/v1/_leases/release",
		body: `{"lease_id":"0123456789abcdef0123456789abcdef"}`},
	// No event stream is open, so every caller gets the unknown-stream answer.
	{slug: "live-subscriptions", method: http.MethodPut, path: "/api/v1/_events/nope/subscriptions",
		body: `{"subscriptions":[{"key":"k","path":"/api/v1/tickets/TKT-1"}]}`},
	{slug: "conflict-resolve", method: http.MethodPost, path: "/api/v1/_conflicts/resolve",
		body: `{"path":"entities/ticket/TKT-4.md","property_choices":{"secret":"theirs"}}`},
}

// goldenLogo is the logo the theme upload cases send.
const goldenLogo = `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`

// goldenCalDAVTodo returns the to-do a CalDAV client writes back for a
// ticket it was served.
func goldenCalDAVTodo(id, summary string) string {
	return strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//golden//EN",
		"BEGIN:VTODO", "UID:ticket--" + id + "@rela", "DTSTAMP:20250101T000000Z",
		"SUMMARY:" + summary, "STATUS:NEEDS-ACTION", "END:VTODO", "END:VCALENDAR", "",
	}, "\r\n")
}

// goldenMCPAccept is the Accept header a streamable-HTTP MCP client sends.
const goldenMCPAccept = "application/json, text/event-stream"

// goldenMCPCall returns a JSON-RPC tools/call request for tool.
func goldenMCPCall(tool, arguments string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool +
		`","arguments":` + arguments + `}}`
}

// goldenMCP are the remote MCP calls. The endpoint is mounted over the JWT
// gate only, so they run in that variant, for every kind of principal.
var goldenMCP = []goldenEndpoint{
	{slug: "mcp-show", method: http.MethodPost, path: "/api/v1/_mcp", accept: goldenMCPAccept,
		body: goldenMCPCall("show_entity", `{"id":"TKT-2"}`)},
	{slug: "mcp-show-hidden", method: http.MethodPost, path: "/api/v1/_mcp", accept: goldenMCPAccept,
		body: goldenMCPCall("show_entity", `{"id":"TKT-4"}`)},
	{slug: "mcp-list", method: http.MethodPost, path: "/api/v1/_mcp", accept: goldenMCPAccept,
		body: goldenMCPCall("list_entities", `{"type":"ticket"}`)},
	// The editor may not write note; compare update-denied-field.
	{slug: "BUG-VUNO11-mcp-update-denied-field", method: http.MethodPost, path: "/api/v1/_mcp", accept: goldenMCPAccept,
		body: goldenMCPCall("update_entity", `{"id":"TKT-3","properties":{"note":"edited"}}`)},
}

// goldenEndpoints returns every endpoint of the corpus.
func goldenEndpoints() []goldenEndpoint {
	return slices.Concat(goldenReads, goldenWrites, goldenMCP)
}

// goldenSwitchSlugs are the endpoints whose answer depends on the kind of
// ACL: the type switches on the ACL implementation (gated UI elements,
// elevated documents, privileged history reveal, comment writes, command
// authorization), plus the plain read and write paths for comparison.
var goldenSwitchSlugs = []string{
	"sidebar", "dashboard", "document", "document-elevated", "history-version",
	"comments", "comment-add", "commands", "command-entity", "BUG-2TPO2F-command-list-input",
	"list", "item", "view", "me", "create", "update", "update-denied-field", "delete",
	"action-transition",
}

// goldenJWTRefusalSlugs are the endpoints the JWT variant also sends with a
// forged and with a missing assertion, to show both are refused before any
// handler runs.
var goldenJWTRefusalSlugs = []string{"me", "list", "item", "search-title", "update"}

// goldenCases expands the endpoint tables into the corpus.
func goldenCases() []goldenCase {
	var out []goldenCase
	add := func(v goldenVariant, e goldenEndpoint, as, assertion string) {
		out = append(out, goldenCase{
			goldenEndpoint: e, variant: v, as: as, assertion: assertion,
			shared: e.principalFree && as != "" && assertion == "",
		})
	}
	all := slices.Concat(goldenReads, goldenWrites)

	// Every endpoint for every kind of principal: the four people, a login
	// with no person, and no identity at all.
	for _, e := range all {
		for _, as := range []string{goldenAdmin, goldenEditor, goldenViewer, goldenOutsider, goldenUnmatched, ""} {
			add(goldenDeclHeader, e, as, "")
		}
	}
	for _, e := range goldenMCP {
		for _, as := range []string{goldenAdmin, goldenEditor, goldenViewer, goldenUnmatched} {
			add(goldenDeclJWT, e, as, "")
		}
	}
	// Under the JWT gate, every read for a matched principal and for the
	// one `unmatched_principal: reject` applies to.
	for _, e := range goldenReads {
		add(goldenDeclJWT, e, goldenViewer, "")
		add(goldenDeclJWT, e, goldenUnmatched, "")
	}
	for _, e := range all {
		if slices.Contains(goldenJWTRefusalSlugs, e.slug) {
			if !slices.ContainsFunc(goldenReads, func(r goldenEndpoint) bool { return r.slug == e.slug }) {
				add(goldenDeclJWT, e, goldenViewer, "")
				add(goldenDeclJWT, e, goldenUnmatched, "")
			}
			add(goldenDeclJWT, e, goldenViewer, "unsigned:"+goldenViewer)
			add(goldenDeclJWT, e, "", "")
		}
		if slices.Contains(goldenSwitchSlugs, e.slug) {
			for _, v := range []goldenVariant{goldenNopVariant, goldenROVariant} {
				add(v, e, goldenEditor, "")
			}
		}
	}
	return out
}

// recordGoldenCase runs c on a fresh app and renders the recording.
func recordGoldenCase(t *testing.T, c goldenCase) string {
	t.Helper()
	g := newGoldenApp(t, c.variant)
	req := goldenRequest(t, c)
	rec := httptest.NewRecorder()

	serveFailingOnPanic(t, g.router, rec, req)
	window := timeWindow{from: g.builtFrom.Add(-time.Second), to: time.Now().Add(time.Second)}

	var b strings.Builder
	if c.anonymous() {
		b.WriteString("request: every corpus request\n")
	} else {
		fmt.Fprintf(&b, "request: %s %s\n", c.method, c.path)
	}
	fmt.Fprintf(&b, "identity: %s\n", goldenIdentityLine(c))
	fmt.Fprintf(&b, "status: %d\n", rec.Code)
	for _, h := range goldenHeaders {
		if v := rec.Header().Values(h); len(v) > 0 {
			fmt.Fprintf(&b, "header %s: %s\n", h, window.normalize(strings.Join(v, ", ")))
		}
	}
	fmt.Fprintf(&b, "reads: %s\n", g.calls.String())
	fmt.Fprintf(&b, "breadth: %s\n", g.breadth.String())
	fmt.Fprintf(&b, "bodies: %s\n", g.bodies.String())
	fmt.Fprintf(&b, "versions: %s\n", goldenVersionLine(g.versions.snapshot()))
	raw := rec.Body.Bytes()
	if len(c.unorderedArrays) > 0 && !c.anonymous() {
		sorted, err := sortTopLevelArrays(raw, c.unorderedArrays)
		if err != nil {
			t.Fatalf("sort %v: %v", c.unorderedArrays, err)
		}
		raw = sorted
		fmt.Fprintf(&b, "body (%s sorted):\n", strings.Join(c.unorderedArrays, ", "))
	} else {
		b.WriteString("body:\n")
	}
	body := window.normalize(goldenBody(rec.Header().Get("Content-Type"), raw))
	if c.minted != nil {
		body = c.minted.ReplaceAllString(body, "<minted>")
	}
	if c.anonymous() {
		body = goldenInstance.ReplaceAllString(body, `"instance": "<request path>"`)
	}
	b.WriteString(body)
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

// serveFailingOnPanic serves req and fails the case if a handler panics. No
// supported wiring panics, so a panic is a defect, never an answer to record.
func serveFailingOnPanic(t *testing.T, h http.Handler, w http.ResponseWriter, req *http.Request) {
	t.Helper()
	defer func() {
		if v := recover(); v != nil {
			t.Fatalf("handler panicked: %v\n%s", v, debug.Stack())
		}
	}()
	h.ServeHTTP(w, req)
}

// sortTopLevelArrays returns the JSON object raw with the elements of each
// array member named in keys sorted by [sortByName]. Every other member
// keeps its bytes and its place.
func sortTopLevelArrays(raw []byte, keys []string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, errors.New("body is not a JSON object")
	}
	var out bytes.Buffer
	out.WriteByte('{')
	for i := 0; dec.More(); i++ {
		keyTok, keyErr := dec.Token()
		if keyErr != nil {
			return nil, keyErr
		}
		name, _ := keyTok.(string)
		var value json.RawMessage
		if decErr := dec.Decode(&value); decErr != nil {
			return nil, decErr
		}
		if slices.Contains(keys, name) {
			if value, err = sortByName(value); err != nil {
				return nil, err
			}
		}
		if i > 0 {
			out.WriteByte(',')
		}
		quoted, quoteErr := json.Marshal(name)
		if quoteErr != nil {
			return nil, quoteErr
		}
		out.Write(quoted)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// sortByName sorts a JSON array of objects by their "name" member, or a
// JSON array of strings by value.
func sortByName(raw json.RawMessage) (json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, item := range items {
		if json.Unmarshal(item, &names[i]) == nil {
			continue
		}
		var named struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(item, &named); err != nil {
			return nil, err
		}
		names[i] = named.Name
	}
	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return names[order[a]] < names[order[b]] })
	sorted := make([]json.RawMessage, len(items))
	for i, k := range order {
		sorted[i] = items[k]
	}
	return json.Marshal(sorted)
}

// goldenHeaders are the response headers a recording keeps. They are the
// ones a list, a write or a refusal carries meaning in; the rest are
// transport detail.
var goldenHeaders = []string{"X-Total-Count", "Link", "ETag", "Location", "Allow", "WWW-Authenticate"}

func goldenRequest(t *testing.T, c goldenCase) *http.Request {
	t.Helper()
	var body *bytes.Buffer
	contentType := ""
	switch {
	case c.multipart:
		body = &bytes.Buffer{}
		mw := multipart.NewWriter(body)
		if err := mw.SetBoundary("golden-boundary"); err != nil {
			t.Fatal(err)
		}
		field, fileName, _ := strings.Cut(cmp.Or(c.formFile, "file/new.txt"), "/")
		fw, err := mw.CreateFormFile(field, fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(c.body)); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		contentType = mw.FormDataContentType()
	case c.body != "":
		body = bytes.NewBufferString(c.body)
		contentType = cmp.Or(c.contentType, "application/json")
	default:
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequestWithContext(t.Context(), c.method, c.path, body)
	// The SPA's shape: same-origin, so the CSRF check passes as it does for
	// the browser client.
	req.Host = goldenHost
	req.Header.Set("Origin", "http://"+goldenHost)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.accept != "" {
		req.Header.Set("Accept", c.accept)
	}
	switch {
	case c.variant.jwt && c.assertion != "":
		req.Header.Set(goldenAssertionHeader, c.assertion)
	case c.variant.jwt && c.as != "":
		req.Header.Set(goldenAssertionHeader, "signed:"+c.as)
	case c.as != "":
		req.Header.Set(goldenUserHeader, c.as)
	}
	return req
}

func goldenIdentityLine(c goldenCase) string {
	switch {
	case c.shared && c.variant.jwt:
		return "a signed assertion for each authenticated principal, all answered alike"
	case c.shared:
		return "a header login for each authenticated principal, all answered alike"
	case c.assertion != "":
		return "assertion " + c.assertion
	case c.as == "":
		return "none"
	case c.variant.jwt:
		return "signed assertion for " + c.as
	default:
		return "header " + c.as
	}
}

func goldenVersionLine(calls map[string]int) string {
	keys := make([]string, 0, len(calls))
	for k := range calls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, calls[k]))
	}
	return strings.Join(parts, " ")
}

// goldenBody renders a response body: JSON indented in its own key order,
// anything else as sent.
func goldenBody(contentType string, raw []byte) string {
	if strings.Contains(contentType, "json") {
		var out bytes.Buffer
		if err := json.Indent(&out, raw, "", "  "); err == nil {
			return out.String()
		}
	}
	return string(raw)
}

// timeWindow is the wall-clock span of one case, from the start of the app
// build to the end of the request. A timestamp inside it was minted by the
// run (the store stamps rows as the fixture seeds them, a write stamps its
// own), so its value is not part of the contract. A timestamp outside it is
// one the fixture fixed, far from now, and is kept.
type timeWindow struct{ from, to time.Time }

// goldenTimestamp matches the timestamp forms the API emits: RFC 3339 with
// optional fraction, and the iCalendar UTC form.
var goldenTimestamp = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})|\d{8}T\d{6}Z`)

// normalize replaces every timestamp inside w with a placeholder, by value.
func (w timeWindow) normalize(s string) string {
	return goldenTimestamp.ReplaceAllStringFunc(s, func(m string) string {
		ts, err := time.Parse(time.RFC3339Nano, m)
		if err != nil {
			ts, err = time.Parse("20060102T150405Z", m)
		}
		if err != nil || ts.Before(w.from.Truncate(time.Second)) || ts.After(w.to) {
			return m
		}
		return "<now>"
	})
}

// checkGolden compares got with the recording at file, or rewrites it under
// -update. A mismatch prints a line diff.
func checkGolden(t *testing.T, file, got string) {
	t.Helper()
	if *goldenUpdate {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v\nRecord it with: go test ./internal/dataentry -run TestGoldenParity -update", file, err)
	}
	if string(want) == got {
		return
	}
	t.Errorf("wire output differs from %s (- recorded, + now):\n%s\n"+
		"If the change is intended, rewrite with: go test ./internal/dataentry -run TestGoldenParity -update",
		file, lineDiff(string(want), got))
}

// checkNoStaleGoldens fails on a recording no case produces, or removes it
// under -update, so a renamed or dropped case cannot leave a file behind
// that nothing checks.
func checkNoStaleGoldens(t *testing.T, produced map[string]bool) {
	t.Helper()
	err := filepath.WalkDir(goldenDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".golden") {
			return nil
		}
		rel, err := filepath.Rel(goldenDir, p)
		if err != nil {
			return err
		}
		if produced[rel] {
			return nil
		}
		if *goldenUpdate {
			return os.Remove(p)
		}
		t.Errorf("stale recording %s: no corpus case produces it", p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// lineDiff renders a minimal line diff of want and got, with two lines of
// context around each change.
func lineDiff(want, got string) string {
	a := strings.Split(want, "\n")
	b := strings.Split(got, "\n")
	// Longest common subsequence table, from the end.
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type line struct {
		op   byte
		text string
	}
	var lines []line
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			lines = append(lines, line{' ', a[i]})
			i++
			j++
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			lines = append(lines, line{'+', b[j]})
			j++
		default:
			lines = append(lines, line{'-', a[i]})
			i++
		}
	}
	const context = 2
	var out strings.Builder
	lastShown := -1
	for k, l := range lines {
		if l.op == ' ' {
			near := false
			for d := -context; d <= context; d++ {
				if n := k + d; n >= 0 && n < len(lines) && lines[n].op != ' ' {
					near = true
					break
				}
			}
			if !near {
				continue
			}
		}
		if lastShown >= 0 && k > lastShown+1 {
			out.WriteString("  ...\n")
		}
		out.WriteByte(l.op)
		out.WriteByte(' ')
		out.WriteString(l.text)
		out.WriteByte('\n')
		lastShown = k
	}
	return out.String()
}
