// Package mail sends outbound email.
//
// It owns three things: the [Sender] seam and its transports, the
// .rela/mail.yaml configuration, and a best-effort [Outbox] that keeps delivery
// off the caller's goroutine.
//
// # Delivery is BEST-EFFORT
//
// The outbox is an in-process buffer, not a durable queue. A crash or restart
// with undelivered mail LOSES that mail. This is deliberate for the current
// scope, not an oversight — but the consequence has to be stated plainly rather
// than implied:
//
//   - In rela-server there is no signal handler, so Services.Close never runs
//     and every pending message is lost on every restart, with no drain at all.
//   - The drain path is real only where Close genuinely runs: CLI commands, the
//     desktop app, and multi-tenant eviction.
//
// So mail here is NOTIFICATION, never a system of record. Nothing may be built
// on an assumed delivery guarantee — if a caller needs to know a message
// arrived, this package cannot tell it. A durable queue with swappable backends
// (IDEA-WIJ2H1) is the planned successor, at which point the outbox becomes its
// first consumer and this caveat is retired.
//
// # Header injection is this package's job
//
// Callers supply the subject and addresses. Both are validated here, at
// enqueue, and CR/LF is REJECTED rather than escaped. That is deliberate: the
// SMTP library rejects CRLF in addresses but accepts it in a subject, where it
// is neutralized only incidentally by RFC 2047 encoded-word escaping. Relying
// on an encoding side effect for a security property is how it breaks quietly
// on a library upgrade, so the check lives here and applies to every
// caller-supplied header value.
//
// Nil: [NewOutbox] and the transport constructors reject nil required
// collaborators rather than substituting a no-op.
package mail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Sender delivers one message.
//
// Implementations must be safe for concurrent use and must NOT mutate the
// message they are handed: the outbox may hand the same message to a second
// attempt after a failure, and a transport that consumed or rewrote it would
// make the retry send something different from the original.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Address is one recipient.
type Address struct {
	// Email is the address. Required.
	Email string

	// Name is an optional display name.
	Name string
}

// Message is a rendered email, ready to transmit.
//
// The body parts arrive already rendered (see internal/mailrender). A transport
// never renders, so two transports cannot disagree about what a message looks
// like, and a retry cannot re-render against changed data.
type Message struct {
	// To are the recipients. At least one is required.
	To []Address

	// Subject is the mail subject.
	Subject string

	// HTML is the text/html part. At least one of HTML or Text is required.
	HTML []byte

	// Text is the text/plain part. Always set it: a message without a plain
	// alternative is both less legible and a spam signal.
	Text []byte

	// InlineImages are parts referenced from the HTML by cid:.
	InlineImages []InlineImage

	// RenderedFor names the principal whose visibility the content was
	// rendered under.
	//
	// This package does not enforce anything with it — there is no trigger
	// here yet. It exists because mail leaves the ACL perimeter irrevocably,
	// and the ticket that adds per-recipient scoping (TKT-U2R7GU) needs a
	// place to attach that gate. Without a field naming the principal, the
	// only place left to check would be the call site, which is exactly the
	// per-consumer redaction the project forbids. Ship the seam with the
	// feature, not after it.
	RenderedFor string
}

// InlineImage is an image part referenced from the HTML body by Content-ID.
type InlineImage struct {
	// CID is the Content-ID, referenced as cid:<CID> in the HTML.
	CID string

	// ContentType is the MIME type, e.g. "image/png".
	ContentType string

	// Data is the raw image.
	Data []byte
}

// allowedInlineTypes is the raster allowlist for embedded images.
//
// An allowlist rather than an SVG denylist: the question is which formats mail
// clients render safely, and that set is small and stable, while the set of
// active-content formats is not.
var allowedInlineTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

const allowedInlineTypeList = "image/png, image/jpeg, image/gif, image/webp"

// ErrOutboxFull is returned by Enqueue when the buffer is at capacity.
//
// Returned rather than dropped silently: a full buffer means the mail server is
// unreachable and a backlog is building, which is an operational condition the
// caller may want to act on. A log line in a server nobody tails is not a
// signal.
var ErrOutboxFull = errors.New("mail: outbox full")

// Validate checks that the message is well formed and header-safe.
func (m *Message) Validate() error {
	if len(m.To) == 0 {
		return errors.New("mail: no recipients")
	}
	for i, a := range m.To {
		if strings.TrimSpace(a.Email) == "" {
			return fmt.Errorf("mail: recipient %d has no address", i)
		}
		if err := validateHeaderValue("recipient", a.Email); err != nil {
			return err
		}
		if err := validateHeaderValue("recipient name", a.Name); err != nil {
			return err
		}
	}
	if err := validateHeaderValue("subject", m.Subject); err != nil {
		return err
	}
	if len(m.HTML) == 0 && len(m.Text) == 0 {
		return errors.New("mail: message has no body")
	}
	for i, img := range m.InlineImages {
		if strings.TrimSpace(img.CID) == "" {
			return fmt.Errorf("mail: inline image %d has no CID", i)
		}
		if err := validateHeaderValue("inline image CID", img.CID); err != nil {
			return err
		}
		if !allowedInlineTypes[strings.ToLower(strings.TrimSpace(img.ContentType))] {
			// Enforced, not merely documented. SVG in particular is
			// script-capable and unsupported by mail clients, so embedding an
			// operator-uploaded one would ship active content into inboxes.
			return fmt.Errorf("mail: inline image %q has content type %q; "+
				"only %s are allowed", img.CID, img.ContentType, allowedInlineTypeList)
		}
	}
	return nil
}

// validateHeaderValue rejects characters that could forge a header.
//
// CR and LF are the classic injection vector: a newline in a subject or address
// ends the header and starts an attacker-chosen one (a Bcc, typically). NUL is
// rejected too — it terminates strings in the C libraries downstream of an SMTP
// server, so a value containing one can be interpreted differently by different
// hops.
//
// Rejecting beats sanitizing: a caller passing a newline in a subject has a
// bug, and silently rewriting the value hides it.
func validateHeaderValue(field, v string) error {
	if i := strings.IndexAny(v, "\r\n\x00"); i >= 0 {
		return fmt.Errorf("mail: %s contains an illegal control character at offset %d "+
			"(CR, LF and NUL are refused: they can forge headers)", field, i)
	}
	return nil
}

// redact replaces every non-empty secret with a placeholder.
//
// Applied where an error or log line could carry a credential. Errors from an
// SMTP server often echo part of the exchange, and `rela validate` prints
// errors verbatim, so this is the backstop for the cases the structure of the
// code does not already prevent.
func redact(s string, secrets ...string) string {
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		// The plaintext, plus the encodings a server can echo back. AUTH PLAIN
		// and AUTH LOGIN put base64 on the wire, so a 535 response quoting the
		// offending line contains the credential in that form and a plaintext
		// substring match would sail straight past it.
		for _, form := range encodedForms(sec) {
			s = strings.ReplaceAll(s, form, redactedPlaceholder)
		}
	}
	return s
}

// redactedPlaceholder is what replaces a credential in any string bound for a
// log or an error.
const redactedPlaceholder = "<REDACTED>"

// encodedForms returns the representations of sec that could appear in an SMTP
// server's response: the plaintext, its base64 encoding (AUTH LOGIN sends the
// password alone), and the AUTH PLAIN payload's encoding of it.
//
// Longest first, so replacing a shorter form cannot corrupt a longer one that
// contains it.
func encodedForms(sec string) []string {
	forms := []string{
		base64.StdEncoding.EncodeToString([]byte("\x00" + sec)),
		base64.StdEncoding.EncodeToString([]byte(sec)),
		sec,
	}
	sort.Slice(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	return forms
}
