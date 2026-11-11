package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// tenantFlags switch tenant mode on and bound it (TKT-HF5KE12). Tenant mode
// is an explicit switch, never inferred from a file being present.
type tenantFlags struct {
	enabled bool
	// maxResident and maxOpen are the registry bounds; zero takes the
	// registry's defaults.
	maxResident, maxOpen int
}

// coverage-ignore-start: main-or-wiring: registers flags on the global flag.CommandLine
func registerTenantFlags(t *tenantFlags) {
	flag.BoolVar(&t.enabled, "tenant-mode", tenant.ModeEnabled(os.Getenv),
		"Serve several organizations, each from its own PostgreSQL schema derived from the verified "+
			"org_id (postgres build only). Also enabled by "+tenant.EnvTenantMode+"=1.")
	flag.IntVar(&t.maxResident, "tenant-max-resident", envInt("RELA_TENANT_MAX_RESIDENT"),
		"Tenant mode: idle tenants kept open; each holds its own connection pool (default 4). "+
			"Also RELA_TENANT_MAX_RESIDENT.")
	flag.IntVar(&t.maxOpen, "tenant-max-open", envInt("RELA_TENANT_MAX_OPEN"),
		"Tenant mode: hard ceiling on open tenants (default twice -tenant-max-resident). "+
			"Also RELA_TENANT_MAX_OPEN.")
}

// coverage-ignore-end

// envInt returns $key as an int, or 0 when unset or not a number. An
// unparsable value takes the default rather than failing: the flag can
// still override it, and the bounds have safe defaults.
func envInt(key string) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return 0
	}
	return n
}

// refuseTenantDeployment is the check a server without tenant mode runs
// before it opens its one store. It refuses a project that still holds the
// old tenant map file, and a database that serves tenants, so a deployment
// that forgot to switch tenant mode on fails closed instead of serving
// every org from `public`.
func refuseTenantDeployment(ctx context.Context, projectDir string, getenv func(string) string) error {
	root, err := appbuild.DiscoverRoot(projectDir)
	if err != nil {
		return err
	}
	if err := tenant.RefuseLegacyMap(root); err != nil {
		return err
	}
	if dsn := getenv("RELA_DATABASE_URL"); dsn != "" {
		return tenant.RefuseTenantDatabase(ctx, dsn)
	}
	return nil
}

// validateTenantMode refuses a tenant-mode configuration the host cannot
// serve safely. supported is [tenant.Supported], passed in so the check is
// testable on every build.
//
// Tenant mode routes by the verified `org_id`, so it needs JWT identity: a
// proxy header or $RELA_DATAENTRY_USER carries no verified org.
// validateIdentityFlags already refuses those next to -jwt-*. Configure is
// refused because a save rebuilds one generation over one store, and tenant
// mode has many.
func validateTenantMode(f *serverFlags, envUser string, supported bool) (identityMode, error) {
	if !supported {
		return identityHeader, tenant.ErrUnsupportedBuild
	}
	mode, err := validateIdentityFlags(f, envUser)
	if err != nil {
		return mode, fmt.Errorf("invalid identity configuration: %w", err)
	}
	if mode != identityJWT {
		return mode, errors.New("tenant mode needs JWT identity (-jwt-issuer, -jwt-audience, -jwt-jwks-url): " +
			"the tenant is chosen by the verified org_id claim")
	}
	if f.configEditing {
		return mode, errors.New("--config-editing is not supported in tenant mode")
	}
	return mode, nil
}

// tenantResident is one open tenant: its services, the data-entry app over
// them and the background workers that keep the app's live features
// running. The registry keeps it while the tenant is resident and closes it
// on eviction or shutdown.
type tenantResident struct {
	t       tenant.Tenant
	opened  *tenant.Opened
	app     *dataentry.App
	handler http.Handler
	relay   *dataentry.CollabRelay
	stop    context.CancelFunc
	done    <-chan struct{}

	// retire ends the app's streams once: [tenant.Registry] may call Revoke while
	// the last release runs Close, and both end them.
	retire sync.Once
}

// Revoke ends the app's live-update and co-editing streams when the tenant
// stops being served, so the requests holding it finish and it can close.
func (r *tenantResident) Revoke() { r.retire.Do(func() { dataentry.Retire(r.app) }) }

// Close tears the tenant down in dependency order: end the app's streams
// and watchers, stop the workers that read the services, close the
// services (which closes the tenant's audit sink last) and release the
// tenant's schema lock, then close the co-editing connections.
func (r *tenantResident) Close() error {
	r.Revoke()
	r.stop()
	<-r.done
	err := r.opened.Close()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if cerr := r.relay.Shutdown(ctx); cerr != nil {
		slog.Warn("tenant: closing co-editing connections", "org_id", r.t.OrgID, "schema", r.t.Schema, "error", cerr)
	}
	return err
}

// openTenant builds the resident for one opened tenant. It takes ownership
// of opened: on an error opened is closed.
func (s *server) openTenant(t tenant.Tenant, opened *tenant.Opened) (*tenantResident, error) {
	svc := opened.Services
	relay := dataentry.NewCollabRelay(dataentry.EmbeddedBuildID())
	// The tenant's own verifiers accept only its own org. The host already
	// routes by the verified org; this keeps a tenant app from acting for
	// another org whatever reached it.
	ident := appIdentity{assertion: tenanthost.ForOrg(s.assertion, t.OrgID)}
	if s.webhook != nil {
		ident.webhook = tenanthost.WebhookForOrg(webhookVerifierAdapter{s.webhook}, t.OrgID)
	}
	app, err := s.buildApp(svc, relay, ident)
	if err != nil {
		if cerr := opened.Close(); cerr != nil {
			slog.Warn("tenant: closing services after a failed open", "org_id", t.OrgID, "error", cerr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		_ = relay.Shutdown(ctx)
		return nil, err
	}
	if len(app.Cfg().Webhooks) > 0 {
		// A declarative hook carries no assertion, so the host cannot tell
		// which tenant it is for and refuses it.
		slog.Warn("tenant mode: declarative webhooks need a verified assertion and are refused without one",
			"org_id", t.OrgID, "schema", t.Schema, "hooks", len(app.Cfg().Webhooks))
	}
	if err := app.StartWatching(); err != nil {
		slog.Warn("tenant: file watcher not started", "org_id", t.OrgID, "schema", t.Schema, "error", err)
	}
	handler := app.NewRouter(dataentry.WithAccessLog(s.accessLog), dataentry.WithQueryCeiling(s.f.queryCeiling))

	// The scheduler is not started per tenant (TKT-M3SHYE0). The workers
	// below serve live features of the app and read only this tenant's
	// services.
	ctx, cancel := context.WithCancel(context.Background())
	leases := appbuild.WatchLeases(ctx, svc)
	worker := appbuild.RunRealtimeWorker(ctx, svc)
	follower := appbuild.FollowChannels(ctx, svc)
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-leases
		<-worker
		<-follower
	}()
	slog.Info("tenant opened", "org_id", t.OrgID, "schema", t.Schema)
	return &tenantResident{
		t: t, opened: opened, app: app, handler: handler, relay: relay, stop: cancel, done: done,
	}, nil
}

// tenantLimits are the registry bounds; zero takes the registry's defaults.
type tenantLimits struct {
	resident, open int
}

// tenantHost is a running multi-tenant server: the front handler and the
// registry it leases tenants from.
type tenantHost struct {
	addr     string
	handler  http.Handler
	registry *tenant.Registry[*tenantResident]
	// resolver is closed after the registry.
	resolver io.Closer
}

// close closes the registry, then the resolver it reads.
func (th *tenantHost) close() {
	if err := th.registry.Close(); err != nil {
		slog.Warn("closing tenants", "error", err)
	}
	if th.resolver != nil {
		_ = th.resolver.Close()
	}
}

// revalidate re-resolves the open tenants every interval until ctx ends.
func (th *tenantHost) revalidate(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			th.registry.Revalidate(ctx)
		}
	}
}

// newTenantHost builds tenant mode. Tenants are found through the control
// table in the database $RELA_DATABASE_URL names; the DSN is read from the
// environment only.
func newTenantHost(f *serverFlags, accessLog *slog.Logger) (*tenantHost, error) {
	mode, err := validateTenantMode(f, os.Getenv(dataentry.EnvDataEntryUserVar), tenant.Supported)
	if err != nil {
		return nil, err
	}
	absDir, err := filepath.Abs(f.projectDir)
	if err != nil {
		return nil, fmt.Errorf("invalid project dir: %w", err)
	}
	root, err := appbuild.DiscoverRoot(absDir)
	if err != nil {
		return nil, err
	}
	if legacyErr := tenant.RefuseLegacyMap(root); legacyErr != nil {
		return nil, legacyErr
	}
	ctx := context.Background()
	resolver, err := tenant.NewClusterResolver(ctx, os.Getenv("RELA_DATABASE_URL"))
	if err != nil {
		return nil, err
	}
	if err = resolver.Check(ctx); err != nil {
		_ = resolver.Close()
		return nil, err
	}
	base, err := appbuild.DiscoverSharedBase(absDir, script.NewEngine(), discoverOptions(f)...)
	if err != nil {
		_ = resolver.Close()
		return nil, fmt.Errorf("load the shared configuration: %w", err)
	}

	s := &server{f: f, addr: net.JoinHostPort(f.bind, f.port), mode: mode, accessLog: accessLog}
	s.idv = buildIdentityVerifier(ctx, f)
	if s.idv == nil {
		_ = resolver.Close()
		return nil, errors.New("tenant mode: no identity verifier was built")
	}
	adapter := assertionVerifierAdapter{s.idv}
	s.assertion = tenanthost.ReuseAssertion(adapter)
	if s.webhook, err = buildWebhookVerifier(f, s.idv); err != nil {
		_ = resolver.Close()
		return nil, err
	}

	th, err := s.tenantHost(base, resolver, tenantLimits{resident: f.tenant.maxResident, open: f.tenant.maxOpen},
		adapter)
	if err != nil {
		_ = resolver.Close()
		return nil, err
	}
	th.resolver = resolver
	return th, nil
}

// tenantHost wires the registry and the front handler over one shared base.
// verifier is the host's own; s.assertion must already be the reuse wrapper
// around the same verifier, so every tenant's gate trusts what the host
// verified and nothing else.
func (s *server) tenantHost(
	base *appbuild.SharedBase, resolver tenant.Resolver, limits tenantLimits, verifier tenanthost.AssertionVerifier,
) (*tenantHost, error) {
	openSvc, err := tenant.SharedBaseOpener(base, tenant.AuditRoot(base.CacheDir()))
	if err != nil {
		return nil, err
	}
	registry, err := tenant.NewRegistry(resolver, func(ctx context.Context, t tenant.Tenant) (*tenantResident, error) {
		opened, oerr := openSvc(ctx, t)
		if oerr != nil {
			return nil, oerr
		}
		//nolint:contextcheck // the tenant outlives the request that opened it; dataentry.NewApp takes no context
		return s.openTenant(t, opened)
	}, limits.resident, tenant.WithMaxOpen(limits.open))
	if err != nil {
		return nil, err
	}

	hcfg := tenanthost.Config{
		Verifier:   verifier,
		HeaderName: s.f.jwtHeader,
		Static:     dataentry.StaticShell(),
		Acquire: func(ctx context.Context, orgID string) (http.Handler, func(), error) {
			lease, aerr := registry.Acquire(ctx, orgID)
			if aerr != nil {
				return nil, nil, aerr
			}
			return lease.Value().handler, lease.Release, nil
		},
	}
	if s.webhook != nil {
		hcfg.Webhook = webhookVerifierAdapter{s.webhook}
	}
	host, err := tenanthost.New(hcfg)
	if err != nil {
		_ = registry.Close()
		return nil, err
	}
	slog.Info("tenant mode enabled", "max_resident", limits.resident, "max_open", limits.open)
	return &tenantHost{addr: s.addr, handler: host, registry: registry}, nil
}

// runTenantMode serves tenant mode until a signal and returns the exit code.
//
// coverage-ignore-func: process entry point for tenant mode; the decisions
// it acts on are validateTenantMode and the tenanthost tests.
// coverage-ignore-start: main-or-wiring: binds a listener and blocks until a signal
func runTenantMode(f *serverFlags, accessLog *slog.Logger) int {
	th, err := newTenantHost(f, accessLog)
	if err != nil {
		slog.Error("failed to initialize tenant mode", "error", err)
		return 1
	}
	if err := startPprofIfRequested(f.debugPprof); err != nil {
		slog.Error("pprof startup failed", "error", err)
		return 1
	}
	slog.Info("starting server", "mode", "tenant", "addr", "http://"+th.addr)
	ctx, stop := context.WithCancel(context.Background())
	// Resolve open tenants again on a timer, so that a suspended or erasing
	// tenant closes and releases its schema lock even when no request for
	// it arrives.
	revalidated := make(chan struct{})
	go func() {
		defer close(revalidated)
		th.revalidate(ctx, tenant.RevalidateInterval)
	}()
	serveErr := serveUntilSignal(newHTTPServer(th.addr, th.handler))
	stop()
	// Wait for a revalidation pass in progress before the registry and
	// the resolver it uses close.
	<-revalidated
	th.close()
	if serveErr != nil {
		slog.Error("server stopped", "error", serveErr)
		return 1
	}
	return 0
}

// coverage-ignore-end
