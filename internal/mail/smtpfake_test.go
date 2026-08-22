package mail_test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"mime"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a minimal SMTP server: enough of RFC 5321 to accept a message
// from a real client, and no more.
//
// It speaks real STARTTLS with a real (self-signed) certificate rather than
// having the client skip verification. That distinction is the point: if the
// test disabled certificate checking, "STARTTLS mandatory" would be verified
// against a client configured to accept anything, and the guarantee would be
// theater. Here the client validates against a CA pool holding this cert.
type fakeSMTP struct {
	ln        net.Listener
	tlsConfig *tls.Config

	// offerSTARTTLS false makes the server advertise no STARTTLS, so a test
	// can assert the client REFUSES rather than downgrading to plaintext.
	offerSTARTTLS bool

	mu       sync.Mutex
	messages []fakeMessage
	authUser string
	authPass string

	// conns tracks accepted connections so Cleanup can close them.
	//
	// Closing only the listener is not enough: a client that drops a
	// connection without QUIT (go-mail does this on some error paths, and the
	// conformance suite deliberately triggers several) leaves handle() parked
	// in a blocking Read. goleak then fails the whole package — intermittently,
	// depending on scheduling, which is the worst kind of CI failure.
	conns  []net.Conn
	closed bool

	// wg tracks handler goroutines so Cleanup can wait for them to exit
	// rather than racing goleak's snapshot.
	wg sync.WaitGroup
}

type fakeMessage struct {
	From string
	To   []string
	Data string
}

// newFakeSMTP starts a server on localhost and returns it with the CA pool a
// client needs to trust it.
func newFakeSMTP(t *testing.T, offerSTARTTLS bool) (*fakeSMTP, *x509.CertPool) {
	t.Helper()

	cert, pool := selfSignedCert(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	s := &fakeSMTP{
		ln:            ln,
		offerSTARTTLS: offerSTARTTLS,
		tlsConfig:     &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
	}
	go s.serve()
	t.Cleanup(s.close)

	return s, pool
}

func (s *fakeSMTP) addr() string { return s.ln.Addr().String() }

func (s *fakeSMTP) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeSMTP) host() string {
	h, _, _ := net.SplitHostPort(s.addr())
	return h
}

func (s *fakeSMTP) received() []fakeMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]fakeMessage, len(s.messages))
	copy(out, s.messages)
	return out
}

func (s *fakeSMTP) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns = append(s.conns, conn)
		s.wg.Add(1)
		s.mu.Unlock()

		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

// close shuts the listener, drops every live connection, and waits for the
// handlers to exit. Registered via t.Cleanup so no goroutine outlives the test.
func (s *fakeSMTP) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	_ = s.ln.Close()
	for _, c := range conns {
		_ = c.Close()
	}
	s.wg.Wait()
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	br := bufio.NewReader(conn)
	write := func(msg string) error {
		_, err := conn.Write([]byte(msg + "\r\n"))
		return err
	}

	if write("220 fake ESMTP") != nil {
		return
	}

	var cur fakeMessage
	tlsDone := false

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb, rest, _ := strings.Cut(line, " ")

		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			ext := []string{"250-fake"}
			if s.offerSTARTTLS && !tlsDone {
				ext = append(ext, "250-STARTTLS")
			}
			if tlsDone {
				ext = append(ext, "250-AUTH PLAIN LOGIN")
			}
			ext = append(ext, "250 SMTPUTF8")
			if write(strings.Join(ext, "\r\n")) != nil {
				return
			}

		case "STARTTLS":
			if !s.offerSTARTTLS {
				if write("502 not implemented") != nil {
					return
				}
				continue
			}
			if write("220 ready") != nil {
				return
			}
			tconn := tls.Server(conn, s.tlsConfig)
			if err := tconn.Handshake(); err != nil {
				return
			}
			conn = tconn
			br = bufio.NewReader(conn)
			tlsDone = true

		case "AUTH":
			s.recordAuth(rest)
			if write("235 ok") != nil {
				return
			}

		case "MAIL":
			cur = fakeMessage{From: extractAddr(rest)}
			if write("250 ok") != nil {
				return
			}

		case "RCPT":
			cur.To = append(cur.To, extractAddr(rest))
			if write("250 ok") != nil {
				return
			}

		case "DATA":
			if write("354 go ahead") != nil {
				return
			}
			body, err := readDotStream(br)
			if err != nil {
				return
			}
			cur.Data = body
			s.mu.Lock()
			s.messages = append(s.messages, cur)
			s.mu.Unlock()
			cur = fakeMessage{}
			if write("250 queued") != nil {
				return
			}

		case "QUIT":
			_ = write("221 bye")
			return

		case "RSET":
			cur = fakeMessage{}
			if write("250 ok") != nil {
				return
			}

		default:
			if write("250 ok") != nil {
				return
			}
		}
	}
}

// recordAuth captures the credentials the client offered, so a test can assert
// authentication happened without inspecting the wire itself.
func (s *fakeSMTP) recordAuth(rest string) {
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return
	}
	if !strings.EqualFold(fields[0], "PLAIN") {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return
	}
	parts := strings.Split(string(raw), "\x00")
	if len(parts) == 3 {
		s.mu.Lock()
		s.authUser, s.authPass = parts[1], parts[2]
		s.mu.Unlock()
	}
}

func (s *fakeSMTP) credentials() (user, pass string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authUser, s.authPass
}

func readDotStream(br *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", err
		}
		if line == ".\r\n" || line == ".\n" {
			return sb.String(), nil
		}
		sb.WriteString(line)
	}
}

func extractAddr(s string) string {
	if i := strings.Index(s, "<"); i >= 0 {
		if j := strings.Index(s[i:], ">"); j > 0 {
			return s[i+1 : i+j]
		}
	}
	if _, after, ok := strings.Cut(s, ":"); ok {
		return strings.TrimSpace(after)
	}
	return strings.TrimSpace(s)
}

// selfSignedCert mints a certificate valid for 127.0.0.1 and localhost, plus
// the pool that trusts it.
func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "rela mail test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// headerValue extracts a header from a captured DATA stream, unfolding
// continuation lines.
func headerValue(data, name string) string {
	lines := strings.Split(data, "\n")
	prefix := strings.ToLower(name) + ":"
	for i, l := range lines {
		if !strings.HasPrefix(strings.ToLower(l), prefix) {
			continue
		}
		var v strings.Builder
		v.WriteString(strings.TrimSpace(l[len(prefix):]))
		// Unfold: continuation lines start with whitespace.
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if next == "" || (next[0] != ' ' && next[0] != '\t') {
				break
			}
			v.WriteString(strings.TrimRight(strings.TrimLeft(next, " \t"), "\r"))
		}
		return strings.TrimRight(v.String(), "\r")
	}
	return ""
}

// decodeHeader decodes RFC 2047 encoded-words, which is how a non-ASCII subject
// travels. Only the forms go-mail emits are handled.
func decodeHeader(v string) string {
	dec := new(mime.WordDecoder)
	out, err := dec.DecodeHeader(v)
	if err != nil {
		return v
	}
	return out
}

// decodeQuotedPrintableish softly decodes the =XX escapes and soft line breaks
// in a DATA stream, enough for a test to spot a distinctive body string.
func decodeQuotedPrintableish(s string) string {
	s = strings.ReplaceAll(s, "=\r\n", "")
	s = strings.ReplaceAll(s, "=\n", "")
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '=' && i+2 < len(s) {
			if b, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				sb.WriteByte(byte(b))
				i += 2
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}
