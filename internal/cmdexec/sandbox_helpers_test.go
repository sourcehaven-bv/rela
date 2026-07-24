package cmdexec

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// pythonPath finds a python3 to drive the socket/file probes. Python is used
// rather than curl/nc because it reports the failure MODE (exception type), so a
// blocked connect is distinguishable from a refused one.
func pythonPath(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 not available for sandbox probes: %v", err)
	}
	return p
}

// testListener is a real loopback TCP listener the egress test connects to.
type testListener struct {
	net.Listener
	port string
}

func mustListen(t *testing.T) *testListener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split port: %v", err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("bad port %q: %v", port, err)
	}
	return &testListener{Listener: ln, port: port}
}

// acceptLoop drains connections so a successful connect completes promptly.
func acceptLoop(ln *testListener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.Close()
	}
}

// pyStr renders a Go string as a Python string literal for -c scripts.
func pyStr(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}
