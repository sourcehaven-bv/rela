package dataentry

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// PROPPATCH sets properties on a collection. rela stores none of them, but the
// way it says so matters: go-webdav answers a flat `501 Not Implemented`
// (caldav/server.go, PropPatch), and Apple Reminders treats that as a failed
// sync step rather than a refused property.
//
// Observed against remindd/3976 (macOS 26.5.1): it PROPPATCHes calendar-color
// on every discovery cycle, gets 501, and NEVER PROCEEDS to the collection —
// 26 PROPFINDs and 12 PROPPATCHes across ~10 minutes with zero REPORTs and zero
// resources fetched. The account looks connected and simply never syncs.
//
// RFC 4918 §9.2 wants a 207 Multi-Status here, with a per-property status
// inside it. That distinction is the whole fix: "I understood your request and
// refuse this property" lets the client carry on, where "I do not implement
// this method" reads as the server being broken.
//
// Answering 403 Forbidden per-property is deliberate over 409 or 200:
//   - 200 would be a lie. The client would show a color that reverts on the
//     next poll — the silent-revert failure this codebase keeps hunting.
//   - 403 is RFC 4918's answer for a property the server refuses to change,
//     which is exactly true: rela has nowhere per-user to keep it (TKT-LD2D33).

// withPropPatch answers collection PROPPATCH before go-webdav can 501 it.
//
// Scoped to the CalDAV prefix and to PROPPATCH; everything else passes through
// untouched, so go-webdav remains the single implementation of the protocol.
func (c *caldavRoutes) withPropPatch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPPATCH" || !strings.HasPrefix(r.URL.Path, caldavPathPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read request body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		names := propPatchNames(body)
		if len(names) == 0 {
			// Nothing recognizable to refuse per-property. Let go-webdav answer,
			// so a malformed body keeps its existing (correct) error.
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			next.ServeHTTP(w, r)
			return
		}
		writePropPatchRefusal(w, r.URL.Path, names)
	})
}

// propPatchName is one property a PROPPATCH tried to set.
type propPatchName struct {
	Space string
	Local string
}

// propPatchNames lists the properties named inside a DAV:propertyupdate body.
//
// Both <set> and <remove> are collected: the answer is the same either way, and
// a client that removes a property it cannot set is no better off for a 501.
func propPatchNames(body []byte) []propPatchName {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	var (
		names  []propPatchName
		inProp bool
		depth  int
	)
	for {
		tok, err := dec.Token()
		if err != nil {
			return names
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch {
			case el.Name.Space == davNamespace && el.Name.Local == "prop":
				inProp, depth = true, 0
			case inProp && depth == 0:
				names = append(names, propPatchName{Space: el.Name.Space, Local: el.Name.Local})
				depth++
			case inProp:
				depth++
			}
		case xml.EndElement:
			switch {
			case el.Name.Space == davNamespace && el.Name.Local == "prop":
				inProp = false
			case inProp && depth > 0:
				depth--
			}
		}
	}
}

// davNamespace is the WebDAV XML namespace.
const davNamespace = "DAV:"

// writePropPatchRefusal emits a 207 that refuses every named property.
//
// Hand-written rather than built through go-webdav's encoder: the handler
// short-circuits before its machinery, and the document is small and fixed.
func writePropPatchRefusal(w http.ResponseWriter, href string, names []propPatchName) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<multistatus xmlns="DAV:"><response><href>`)
	xmlEscape(&b, href)
	b.WriteString(`</href><propstat><prop>`)
	for _, n := range names {
		b.WriteString(`<`)
		xmlEscape(&b, n.Local)
		if n.Space != "" && n.Space != davNamespace {
			b.WriteString(` xmlns="`)
			xmlEscape(&b, n.Space)
			b.WriteString(`"`)
		}
		b.WriteString(`/>`)
	}
	// 403: understood, and refused. NOT 200 — claiming success for a value we
	// discard makes the client display something that reverts on the next poll.
	b.WriteString(`</prop><status>HTTP/1.1 403 Forbidden</status></propstat></response></multistatus>`)

	out := b.String()
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(http.StatusMultiStatus)
	// #nosec G705 -- not an HTML sink: application/xml under nosniff, and every
	// interpolated value goes through xmlEscape.
	_, _ = io.WriteString(w, out)
}

// xmlEscape writes s with XML metacharacters escaped.
func xmlEscape(b *strings.Builder, s string) {
	_ = xml.EscapeText(escapeWriter{b}, []byte(s))
}

// escapeWriter adapts a strings.Builder to io.Writer for xml.EscapeText.
type escapeWriter struct{ b *strings.Builder }

func (w escapeWriter) Write(p []byte) (int, error) { return w.b.Write(p) }
