package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/tools/scalerun"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
	// principalHeader is the header rela-server is told to read the
	// identity from. Only the driver on loopback sends it.
	principalHeader = "X-Forwarded-User"
	startTimeout    = 3 * time.Minute
	stopTimeout     = 15 * time.Second
	readyPoll       = 500 * time.Millisecond
	probeTimeout    = 2 * time.Second
	// maxScale bounds -scales: scale 10 is about 200k entities, already
	// past what the workflow's time budget allows.
	maxScale = 10
)

type runOptions struct {
	configPath, relaBin, serverBin, project, out, dsn, commit string
	scales                                                    []float64
	port                                                      int
	keep                                                      bool
}

// parseScales reads the -scales list. It must hold base, the scale the
// ratio check compares with; without it the ratio check would silently do
// nothing.
func parseScales(s string, base float64) ([]float64, error) {
	var out []float64
	for part := range strings.SplitSeq(s, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || v <= 0 || v > maxScale {
			return nil, fmt.Errorf("scale %q must be a number in (0, %d]", part, maxScale)
		}
		if slices.Contains(out, v) {
			return nil, fmt.Errorf("scale %s is listed twice", scalerun.FormatScale(v))
		}
		out = append(out, v)
	}
	if !slices.Contains(out, base) {
		return nil, fmt.Errorf("scales must include the ratio base scale %s", scalerun.FormatScale(base))
	}
	return out, nil
}

func run(ctx context.Context, cfg *scalerun.Config, o runOptions) error {
	if err := os.MkdirAll(o.out, dirPerm); err != nil {
		return err
	}
	// A stale results.json would let check report an earlier run's numbers
	// when this one fails before its first scale.
	if err := os.Remove(filepath.Join(o.out, "results.json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	res := &scalerun.Results{Commit: o.commit, Started: time.Now(), Iterations: cfg.Iterations}
	runID := strconv.FormatInt(res.Started.Unix(), 36)
	for _, scale := range o.scales {
		fmt.Printf("==> scale %s\n", scalerun.FormatScale(scale))
		sr, err := runScale(ctx, cfg, o, scale, runID)
		if sr != nil {
			res.Scales = append(res.Scales, *sr)
		}
		// Results so far are written even when a scale fails, so the
		// report can show what was measured.
		if werr := writeJSON(filepath.Join(o.out, "results.json"), res); werr != nil {
			return errors.Join(err, werr)
		}
		if err != nil {
			return fmt.Errorf("scale %s: %w", scalerun.FormatScale(scale), err)
		}
	}
	fmt.Printf("results: %s\n", filepath.Join(o.out, "results.json"))
	return nil
}

func runScale(
	ctx context.Context, cfg *scalerun.Config, o runOptions, scale float64, runID string,
) (*scalerun.ScaleResult, error) {
	tag := "s" + strings.ReplaceAll(scalerun.FormatScale(scale), ".", "_")
	schema := "scalerun_" + tag
	if err := resetSchema(ctx, o.dsn, schema); err != nil {
		return nil, err
	}
	if !o.keep {
		defer func() {
			// The run's context may be canceled already; dropping must still run.
			if err := dropSchema(context.WithoutCancel(ctx), o.dsn, schema); err != nil {
				fmt.Fprintln(os.Stderr, "drop schema:", err)
			}
		}()
	}
	dsn, err := withSearchPath(o.dsn, schema+",public")
	if err != nil {
		return nil, err
	}
	project := filepath.Join(o.out, "project-"+tag)
	if err = copyDir(o.project, project); err != nil {
		return nil, fmt.Errorf("copy project: %w", err)
	}
	sr := &scalerun.ScaleResult{Scale: scale}

	fmt.Println("  seeding")
	start := time.Now()
	seedLog := filepath.Join(o.out, "seed-"+tag+".log")
	out, err := runLogged(ctx, seedLog, []string{"RELA_DATABASE_URL=" + dsn},
		o.relaBin, "--project", project, "dev", "seed", "--scale", scalerun.FormatScale(scale))
	if err != nil {
		return nil, fmt.Errorf("seed (see %s): %w", seedLog, err)
	}
	sr.SeedSeconds = time.Since(start).Seconds()
	sr.Entities, sr.Relations = seededCounts(out)
	fmt.Printf("  seeded %d entities, %d relations in %.0f s\n", sr.Entities, sr.Relations, sr.SeedSeconds)

	base := fmt.Sprintf("http://127.0.0.1:%d", o.port)
	srv, err := startServer(ctx, o, project, dsn, filepath.Join(o.out, "server-"+tag+".log"))
	if err != nil {
		return sr, err
	}
	defer srv.stop()
	d := &scalerun.Driver{
		Base:            base,
		PrincipalHeader: principalHeader,
		Client:          &http.Client{Timeout: 2 * time.Minute},
		RSS:             srv.rss,
		RunID:           runID,
		Log:             os.Stdout,
	}
	start = time.Now()
	if werr := srv.waitReady(ctx, d, cfg.Principals[cfg.FixturesAs]); werr != nil {
		return sr, werr
	}
	sr.StartupSeconds = time.Since(start).Seconds()
	sr.RSSIdleKB, _ = srv.rss()
	if sr.Fixtures, err = d.Fixtures(ctx, cfg.Principals[cfg.FixturesAs]); err != nil {
		return sr, err
	}
	fmt.Printf("  measuring (fixtures %+v)\n", sr.Fixtures)
	sr.Endpoints, err = d.Measure(ctx, cfg, sr.Fixtures)
	sr.RSSPeakKB = sr.RSSIdleKB
	for _, e := range sr.Endpoints {
		sr.RSSPeakKB = max(sr.RSSPeakKB, e.RSSKB)
	}
	if !srv.alive() {
		return sr, fmt.Errorf("rela-server exited during the run; see %s", srv.logPath)
	}
	return sr, err
}

var seededLine = regexp.MustCompile(`(\d+) entities, (\d+) relations in`)

func seededCounts(out []byte) (entities, relations int) {
	m := seededLine.FindSubmatch(out)
	if m == nil {
		return 0, 0
	}
	entities, _ = strconv.Atoi(string(m[1]))
	relations, _ = strconv.Atoi(string(m[2]))
	return entities, relations
}

// resetSchema drops and recreates schema, and makes sure pg_trgm exists:
// pgstore's migrations need it, and it lives in public so every scale's
// schema shares one copy.
func resetSchema(ctx context.Context, dsn, schema string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	ident := pgx.Identifier{schema}.Sanitize()
	for _, stmt := range []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public",
		"DROP SCHEMA IF EXISTS " + ident + " CASCADE",
		"CREATE SCHEMA " + ident,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

func dropSchema(ctx context.Context, dsn, schema string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	return err
}

// withSearchPath pins the connection to schema the way a tenant is pinned
// (docs/postgres-backend.md): search_path as a connection parameter. It
// accepts both the URL and the keyword/value DSN forms.
func withSearchPath(dsn, path string) (string, error) {
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			// Not wrapped: url.Parse quotes its input, password included.
			return "", errors.New("RELA_DATABASE_URL is not a valid URL")
		}
		q := u.Query()
		q.Set("search_path", path)
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	return dsn + " search_path=" + path, nil
}

// runLogged runs a command with extra environment, writes its combined
// output to logPath and returns it.
func runLogged(ctx context.Context, logPath string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if werr := os.WriteFile(logPath, buf.Bytes(), filePerm); werr != nil {
		err = errors.Join(err, werr)
	}
	return buf.Bytes(), err
}

type server struct {
	cmd     *exec.Cmd
	logPath string
	logFile *os.File
	done    chan struct{}
}

func startServer(ctx context.Context, o runOptions, project, dsn, logPath string) (*server, error) {
	// The server logs before it binds, so a busy port would send requests,
	// with forged identity headers, to another process.
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(o.port))
	if conn, err := (&net.Dialer{Timeout: probeTimeout}).DialContext(ctx, "tcp", addr); err == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("port %d is in use; pass -port", o.port)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerm)
	if err != nil {
		return nil, err
	}
	// Not CommandContext: stop sends SIGTERM and waits, so the server
	// shuts down cleanly even when the run is interrupted.
	cmd := exec.Command(o.serverBin, //nolint:noctx // operator-chosen binary; stopped by stop
		"-project", project, "-port", strconv.Itoa(o.port), "-bind", "127.0.0.1",
		"-verbose", "-principal-header", principalHeader)
	cmd.Env = append(os.Environ(), "RELA_DATABASE_URL="+dsn)
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return nil, err
	}
	s := &server{cmd: cmd, logPath: logPath, logFile: f, done: make(chan struct{})}
	go func() {
		// The pipe must be drained before Wait.
		copyLog(f, out)
		_ = cmd.Wait()
		close(s.done)
	}()
	return s, nil
}

// copyLog copies the server's output to w without the per-statement SQL
// lines -verbose adds. Those carry every bound argument, often a list of
// thousands of ids, and made the log of one scale-1 run 240 MB. The
// statement count still reaches the driver through Server-Timing, and the
// `request` lines, which carry it too, are kept.
func copyLog(w io.Writer, r io.Reader) {
	br := bufio.NewReaderSize(r, logBufSize)
	for {
		line, err := br.ReadBytes('\n')
		if !bytes.Contains(line, sqlLogMarker) {
			_, _ = w.Write(line)
		}
		if err != nil {
			return
		}
	}
}

var sqlLogMarker = []byte(` level=DEBUG msg="pgstore:`)

const logBufSize = 64 << 10

func (s *server) alive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// waitReady polls /api/v1/_me as principal until it answers 200.
func (s *server) waitReady(ctx context.Context, d *scalerun.Driver, principal string) error {
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		if !s.alive() {
			return fmt.Errorf("rela-server exited at startup; see %s", s.logPath)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.Base+"/api/v1/_me", http.NoBody)
		if err != nil {
			return err
		}
		req.Header.Set("Origin", d.Base)
		req.Header.Set(principalHeader, principal)
		if resp, err := d.Client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPoll):
		}
	}
	return fmt.Errorf("rela-server not ready after %s; see %s", startTimeout, s.logPath)
}

func (s *server) stop() {
	if s.alive() {
		_ = s.cmd.Process.Signal(os.Interrupt)
		select {
		case <-s.done:
		case <-time.After(stopTimeout):
			_ = s.cmd.Process.Kill()
			<-s.done
		}
	}
	_ = s.logFile.Close()
}

// rss reports the server's resident set size in KiB.
func (s *server) rss() (int64, error) {
	pid := s.cmd.Process.Pid
	if runtime.GOOS == "linux" {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			return 0, err
		}
		for line := range strings.SplitSeq(string(b), "\n") {
			if v, ok := strings.CutPrefix(line, "VmRSS:"); ok {
				return strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "kB")), 10, 64)
			}
		}
		return 0, errors.New("no VmRSS line")
	}
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output() //nolint:noctx // fixed argv
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

func copyDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, dirPerm)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file", path)
		}
		b, err := os.ReadFile(path) //nolint:gosec // walking the operator-chosen project
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, filePerm) //nolint:gosec // under the out dir, from WalkDir
	})
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), filePerm)
}
