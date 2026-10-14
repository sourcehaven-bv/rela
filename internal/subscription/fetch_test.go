package subscription

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestFetch_FollowsSameOriginRedirect(t *testing.T) {
	var auth string
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("X-Key")
		_, _ = w.Write([]byte("ok"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := (&Fetcher{}).fetch(t.Context(), request{
		url: mustURL(t, ts.URL+"/old"), header: http.Header{"X-Key": {"k"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.body) != "ok" || resp.finalURL.Path != "/new" || auth != "k" {
		t.Errorf("body %q, final %v, auth %q", resp.body, resp.finalURL, auth)
	}
}

func TestFetch_RefusesCrossOriginRedirect(t *testing.T) {
	var reached bool
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer other.Close()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/x?token=leak", http.StatusFound)
	}))
	defer ts.Close()

	_, err := (&Fetcher{}).fetch(t.Context(), request{url: mustURL(t, ts.URL+"/feed?token=secret")})
	if err == nil || !strings.Contains(err.Error(), "another origin") {
		t.Fatalf("err = %v, want the redirect refused", err)
	}
	if reached {
		t.Error("the other origin was contacted")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "leak") {
		t.Errorf("err %q carries a query string", err)
	}
}

func TestFetch_StopsAfterMaxRedirects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
	}))
	defer ts.Close()
	_, err := (&Fetcher{}).fetch(t.Context(), request{url: mustURL(t, ts.URL+"/a")})
	if err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Errorf("err = %v, want the redirect loop stopped", err)
	}
}

func TestSameOrigin(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		want     bool
	}{
		{"https://a.example/x", "https://a.example/y", true},
		{"https://a.example/x", "https://A.EXAMPLE:443/y", true},
		{"http://a.example/x", "https://a.example/y", true},
		{"https://a.example/x", "http://a.example/y", false},
		{"https://a.example/x", "https://b.a.example/y", false},
		{"https://a.example/x", "https://a.example:8443/y", false},
		{"http://a.example:8080/x", "https://a.example:8443/y", false},
		{"http://a.example/x", "https://evil.example/y", false},
	} {
		if got := sameOrigin(mustURL(t, tc.from), mustURL(t, tc.to)); got != tc.want {
			t.Errorf("sameOrigin(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestFetch_StatusAndCap(t *testing.T) {
	big := strings.Repeat("a", int(maxBody)+1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone" {
			w.WriteHeader(http.StatusGone)
			return
		}
		_, _ = w.Write([]byte(big))
	}))
	defer ts.Close()

	_, err := (&Fetcher{}).fetch(t.Context(), request{url: mustURL(t, ts.URL+"/gone")})
	if err == nil || !strings.Contains(err.Error(), "status 410") {
		t.Errorf("err = %v, want the status reported", err)
	}
	_, err = (&Fetcher{}).fetch(t.Context(), request{url: mustURL(t, ts.URL+"/big")})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("err = %v, want the cap enforced", err)
	}
}

func TestFetch_TransportErrorHidesQuery(t *testing.T) {
	_, err := (&Fetcher{}).fetch(t.Context(), request{url: mustURL(t, "http://127.0.0.1:1/feed?token=secret")})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("err = %v, want a failure without the query", err)
	}
}

func TestValidator(t *testing.T) {
	for in, want := range map[string]string{
		`"abc"`:                      `"abc"`,
		"W/\"a\"\t":                  "W/\"a\"\t",
		"a\nb":                       "",
		"a\x7fb":                     "",
		strings.Repeat("x", 1025):    "",
		"Wed, 21 Oct 2015 07:28 GMT": "Wed, 21 Oct 2015 07:28 GMT",
	} {
		if got := validator(in); got != want {
			t.Errorf("validator(%q) = %q, want %q", in, got, want)
		}
	}
}
