package docs

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// fakeAPI serves canned responses so the assertion logic is tested without a
// server. Records the requests it saw, so a test can prove `as=` and `method=`
// actually reach the client rather than being silently dropped.
type fakeAPI struct {
	responses map[string]APIResponse
	seen      []APIRequest
	err       error
}

func (f *fakeAPI) Do(_ context.Context, req APIRequest) (APIResponse, error) {
	f.seen = append(f.seen, req)
	if f.err != nil {
		return APIResponse{}, f.err
	}
	if r, ok := f.responses[req.Path]; ok {
		return r, nil
	}
	return APIResponse{Status: 404, Body: `{"code":"not_found"}`}, nil
}

func (f *fakeAPI) Close() error { return nil }

func TestAPIIsland(t *testing.T) {
	ok := APIResponse{Status: 200, Body: `{"id":"POL-1"}`}
	denied := APIResponse{Status: 404, Body: `{"code":"not_found"}`}
	forbidden := APIResponse{Status: 403, Body: `{"code":"world_forbidden"}`}

	tests := []struct {
		name      string
		body      string
		responses map[string]APIResponse
		wantErr   string
	}{
		{
			name:      "a matching status passes",
			body:      `api{path="/a", status=200}`,
			responses: map[string]APIResponse{"/a": ok},
		},
		{
			name:      "a wrong status fails and prints the body",
			body:      `api{path="/a", status=200}`,
			responses: map[string]APIResponse{"/a": forbidden},
			wantErr:   "claimed status: 200\n  actual status:  403",
		},
		{
			name:      "a has= claim passes when the body names it",
			body:      `api{path="/a", status=200, has={"POL-1"}}`,
			responses: map[string]APIResponse{"/a": ok},
		},
		{
			name:      "a has= claim fails when the body does not name it",
			body:      `api{path="/a", status=200, has={"POL-2"}}`,
			responses: map[string]APIResponse{"/a": ok},
			wantErr:   `body must contain: "POL-2"`,
		},
		{
			// The disclosure direction: a face the caller may not read must not
			// be named in a response they ARE allowed to fetch.
			name:      "an absent= claim fails when the body discloses it",
			body:      `api{path="/a", status=200, absent={"POL-1"}}`,
			responses: map[string]APIResponse{"/a": ok},
			wantErr:   "this is a disclosure",
		},
		{
			name:      "an absent= claim passes when the body withholds it",
			body:      `api{path="/a", status=200, absent={"Draft"}}`,
			responses: map[string]APIResponse{"/a": ok},
		},
		{
			name:      "an error code claim passes on the code",
			body:      `api{path="/a", error="world_forbidden"}`,
			responses: map[string]APIResponse{"/a": forbidden},
		},
		{
			// The contract is the code, not the prose. A body with a different
			// code must fail even when the status matches.
			name:      "a wrong error code fails",
			body:      `api{path="/a", error="not_found"}`,
			responses: map[string]APIResponse{"/a": forbidden},
			wantErr:   "claimed error: not_found\n  actual error:  world_forbidden",
		},
		{
			name:      "a body with no code reports none rather than matching",
			body:      `api{path="/a", error="not_found"}`,
			responses: map[string]APIResponse{"/a": {Status: 404, Body: "plain text"}},
			wantErr:   "actual error:  (none)",
		},
		{
			name:    "a claimless call is refused",
			body:    `api{path="/a"}`,
			wantErr: "asserts nothing",
		},
		{
			name:    "a missing path is refused",
			body:    `api{status=200}`,
			wantErr: "`path` is required",
		},
		{
			// The existence-oracle property: a denied read and a genuinely
			// missing one must be the same response.
			name: "identical_to passes when the two responses match",
			body: `api{path="/hidden", as="viewer", identical_to={path="/missing", as="viewer"}}`,
			responses: map[string]APIResponse{
				"/hidden":  denied,
				"/missing": denied,
			},
		},
		{
			name: "identical_to fails when the bodies differ",
			body: `api{path="/hidden", identical_to={path="/missing"}}`,
			responses: map[string]APIResponse{
				"/hidden":  {Status: 404, Body: `{"code":"not_permitted"}`},
				"/missing": denied,
			},
			wantErr: "the two responses differ",
		},
		{
			name: "identical_to fails when only the status differs",
			body: `api{path="/hidden", identical_to={path="/missing"}}`,
			responses: map[string]APIResponse{
				"/hidden":  {Status: 403, Body: `{"code":"not_found"}`},
				"/missing": denied,
			},
			wantErr: "the two responses differ",
		},
		{
			name:      "identical_to without its own path is refused",
			body:      `api{path="/a", identical_to={as="viewer"}}`,
			responses: map[string]APIResponse{"/a": ok},
			wantErr:   "identical_to needs its own `path`",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAPI{responses: tc.responses}
			src := "```rela\n" + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), APIClient: f})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want failure containing %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error does not contain %q:\n%v", tc.wantErr, err)
			}
		})
	}
}

// TestAPIPassesRequestFields proves as=/method=/body= actually reach the
// client. A silently-dropped `as=` would make every ACL assertion run as the
// default principal — passing for the wrong reason.
func TestAPIPassesRequestFields(t *testing.T) {
	f := &fakeAPI{responses: map[string]APIResponse{"/a": {Status: 200}}}
	src := "```rela\n" + `api{path="/a", method="POST", as="viewer", body="{}", status=200}` + "\n```\n"
	if _, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), APIClient: f}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(f.seen) != 1 {
		t.Fatalf("want 1 request, got %d", len(f.seen))
	}
	got := f.seen[0]
	if got.Method != http.MethodPost || got.As != "viewer" || got.Body != "{}" {
		t.Errorf("request fields lost: %+v", got)
	}
}

// TestAPINilClientFailsLoud: a missing client must not silently skip. A skipped
// assertion is indistinguishable from a passing one.
func TestAPINilClientFailsLoud(t *testing.T) {
	src := "```rela\n" + `api{path="/a", status=200}` + "\n```\n"
	_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), APIClientErr: "no server built"})
	if err == nil {
		t.Fatal("want failure when no API client is available")
	}
	if !strings.Contains(err.Error(), "no server built") {
		t.Fatalf("want the reason surfaced, got: %v", err)
	}
}

// TestAPISeedReachesTheRequest: the client must receive the manual's seed, or
// it would serve an empty project and every assertion would be about nothing.
func TestAPISeedReachesTheRequest(t *testing.T) {
	f := &fakeAPI{responses: map[string]APIResponse{"/a": {Status: 200}}}
	src := "```rela\n" + `create("risico", {titel="Leak", kans=1, impact=1, status="todo"})
api{path="/a", status=200}` + "\n```\n"
	if _, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), APIClient: f}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(f.seen) == 0 || len(f.seen[0].Seed) != 1 {
		t.Fatalf("want the seeded entity forwarded, got %+v", f.seen)
	}
}

// TestIdenticalIgnoresInstance: RFC 7807 `instance` echoes the requested URL,
// so two responses to different urls always differ there. Comparing it would
// make identical_to fail on every pair — a check that never passes tests
// nothing. Everything else must still match.
func TestIdenticalIgnoresInstance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     APIResponse
		wantFail bool
	}{
		{
			name: "same problem, different instance urls",
			a:    APIResponse{Status: 404, Body: `{"type":"not_found","title":"Entity not found","instance":"/a"}`},
			b:    APIResponse{Status: 404, Body: `{"type":"not_found","title":"Entity not found","instance":"/b"}`},
		},
		{
			// The leak this assertion exists to catch: one says "not permitted",
			// the other "not found", so the pair reveals that /a exists.
			name:     "different type is still caught",
			a:        APIResponse{Status: 404, Body: `{"type":"not_permitted","instance":"/a"}`},
			b:        APIResponse{Status: 404, Body: `{"type":"not_found","instance":"/b"}`},
			wantFail: true,
		},
		{
			name:     "different status is still caught",
			a:        APIResponse{Status: 403, Body: `{"type":"not_found","instance":"/a"}`},
			b:        APIResponse{Status: 404, Body: `{"type":"not_found","instance":"/b"}`},
			wantFail: true,
		},
		{
			name:     "non-JSON bodies compare verbatim",
			a:        APIResponse{Status: 404, Body: "plain a"},
			b:        APIResponse{Status: 404, Body: "plain b"},
			wantFail: true,
		},
		{
			name: "identical non-JSON bodies pass",
			a:    APIResponse{Status: 404, Body: "not found"},
			b:    APIResponse{Status: 404, Body: "not found"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := checkIdentical("/a", "/b", tc.a, tc.b)
			if tc.wantFail && msg == "" {
				t.Fatal("want the difference reported, got pass")
			}
			if !tc.wantFail && msg != "" {
				t.Fatalf("want pass, got:\n%s", msg)
			}
		})
	}
}

// TestIdenticalComparesOracleHeaders: the body is not the only channel. A
// denied GET that emits an ETag lets a replayed If-None-Match return 304,
// confirming the entity exists — pinned in Go by TestACLGet_ETagSuppressedOnDeny
// and now expressible in a manual.
func TestIdenticalComparesOracleHeaders(t *testing.T) {
	body := `{"type":"not_found","instance":"/x"}`
	tests := []struct {
		name     string
		a, b     APIResponse
		wantFail bool
	}{
		{
			name:     "one response carries an ETag and the other does not",
			a:        APIResponse{Status: 404, Body: body, Header: map[string][]string{"Etag": {`W/"abc"`}}},
			b:        APIResponse{Status: 404, Body: body},
			wantFail: true,
		},
		{
			name:     "differing ETags",
			a:        APIResponse{Status: 404, Body: body, Header: map[string][]string{"Etag": {`W/"a"`}}},
			b:        APIResponse{Status: 404, Body: body, Header: map[string][]string{"Etag": {`W/"b"`}}},
			wantFail: true,
		},
		{
			name: "matching ETags are fine",
			a:    APIResponse{Status: 404, Body: body, Header: map[string][]string{"Etag": {`W/"a"`}}},
			b:    APIResponse{Status: 404, Body: body, Header: map[string][]string{"Etag": {`W/"a"`}}},
		},
		{
			// Only an allowlist is compared: Date and Content-Length vary for
			// reasons that disclose nothing, so comparing everything would make
			// the assertion fail always — the same trap `instance` sets.
			name: "headers that disclose nothing are ignored",
			a: APIResponse{Status: 404, Body: body, Header: map[string][]string{
				"Date": {"Mon, 01 Jan 2035 00:00:00 GMT"}, "Content-Length": {"42"},
			}},
			b: APIResponse{Status: 404, Body: body, Header: map[string][]string{
				"Date": {"Tue, 02 Jan 2035 00:00:00 GMT"}, "Content-Length": {"99"},
			}},
		},
		{
			name: "no headers at all on either side",
			a:    APIResponse{Status: 404, Body: body},
			b:    APIResponse{Status: 404, Body: body},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := checkIdentical("/a", "/b", tc.a, tc.b)
			if tc.wantFail {
				if msg == "" {
					t.Fatal("want the header difference reported, got pass")
				}
				if !strings.Contains(msg, "differing headers") {
					t.Errorf("failure should name the header channel:\n%s", msg)
				}
				return
			}
			if msg != "" {
				t.Fatalf("want pass, got:\n%s", msg)
			}
		})
	}
}
