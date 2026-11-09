package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// tenantsFile is the tenant map. Its presence in the project directory turns
// tenant mode on (TKT-D6MF3SW).
const tenantsFile = tenant.ConfigFileName

// tenantsPath returns the tenant map's path when the project directory has
// one, and "" when it does not.
func tenantsPath(projectDir string) (string, error) {
	return tenant.ConfigPath(projectDir)
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
		return identityHeader, errors.New(tenantsFile + " is present, but tenant mode needs the postgres build " +
			"(rela-server-postgres): this build keeps data in the project directory, so every tenant " +
			"would read the same data")
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
}

// Close tears the tenant down in dependency order: end the app's streams
// and watchers, stop the workers that read the services, close the
// services (which closes the tenant's audit sink last) and release the
// tenant's schema lock, then close the co-editing connections.
func (r *tenantResident) Close() error {
	dataentry.Retire(r.app)
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

// tenantLimits are the registry bounds from the tenant map; zero takes the
// registry's defaults.
type tenantLimits struct {
	resident, open int
}

// tenantHost is a running multi-tenant server: the front handler and the
// registry it leases tenants from.
type tenantHost struct {
	addr     string
	handler  http.Handler
	registry *tenant.Registry[*tenantResident]
}

// newTenantHost builds tenant mode from the tenant map at tenantsYAML.
//
// The base DSN may be left out of the file and given in $RELA_DATABASE_URL
// instead. The file may not carry a password (tenant.LoadConfig refuses
// one), because it sits with the committed project config.
func newTenantHost(f *serverFlags, tenantsYAML string, accessLog *slog.Logger) (*tenantHost, error) {
	mode, err := validateTenantMode(f, os.Getenv(dataentry.EnvDataEntryUserVar), tenant.Supported)
	if err != nil {
		return nil, err
	}
	cfg, err := tenant.LoadDeployment(tenantsYAML, os.Getenv)
	if err != nil {
		return nil, err
	}
	resolver, err := cfg.Resolver()
	if err != nil {
		return nil, err
	}

	absDir, err := filepath.Abs(f.projectDir)
	if err != nil {
		return nil, fmt.Errorf("invalid project dir: %w", err)
	}
	base, err := appbuild.DiscoverSharedBase(absDir, script.NewEngine(), discoverOptions(f)...)
	if err != nil {
		return nil, fmt.Errorf("load the shared configuration: %w", err)
	}

	s := &server{f: f, addr: net.JoinHostPort(f.bind, f.port), mode: mode, accessLog: accessLog}
	s.idv = buildIdentityVerifier(context.Background(), f)
	if s.idv == nil {
		return nil, errors.New("tenant mode: no identity verifier was built")
	}
	adapter := assertionVerifierAdapter{s.idv}
	s.assertion = tenanthost.ReuseAssertion(adapter)
	if s.webhook, err = buildWebhookVerifier(f, s.idv); err != nil {
		return nil, err
	}

	return s.tenantHost(base, resolver, tenantLimits{resident: cfg.MaxResident, open: cfg.MaxOpen}, adapter)
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
func runTenantMode(f *serverFlags, tenantsYAML string, accessLog *slog.Logger) int {
	th, err := newTenantHost(f, tenantsYAML, accessLog)
	if err != nil {
		slog.Error("failed to initialize tenant mode", "error", err)
		return 1
	}
	if err := startPprofIfRequested(f.debugPprof); err != nil {
		slog.Error("pprof startup failed", "error", err)
		return 1
	}
	slog.Info("starting server", "mode", "tenant", "addr", "http://"+th.addr)
	serveErr := serveUntilSignal(newHTTPServer(th.addr, th.handler))
	if err := th.registry.Close(); err != nil {
		slog.Warn("closing tenants", "error", err)
	}
	if serveErr != nil {
		slog.Error("server stopped", "error", serveErr)
		return 1
	}
	return 0
}

// coverage-ignore-end
