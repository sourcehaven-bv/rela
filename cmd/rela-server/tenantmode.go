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
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/idporg"
	"github.com/Sourcehaven-BV/rela/internal/lua"
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
	// provision is the -tenant-provision mode: off, webhook or lazy
	// (TKT-TNPRV8). provisionRate caps provisioning starts per minute; zero
	// takes the host's default.
	provision     string
	provisionRate int
	// idpOrgAPI is the IdP operator API address that org status is read
	// from, so a tenant's suspension follows its org's. Required with the
	// IdP webhook (validateStatusSync). Its secret is
	// $RELA_IDP_OPERATOR_SECRET, never a flag.
	idpOrgAPI string
	// luaHTTPAllow is the -lua-http-allow list: CIDRs, IPs and host:port
	// destinations that tenant mode's strict Lua HTTP policy reopens.
	luaHTTPAllow string
	// sharedHostConfig is -tenant-shared-host-config: share the .rela
	// directory's secrets and ai.yaml with every tenant.
	sharedHostConfig bool
	// apiRole is -tenant-api-role: the role an operator principal needs to
	// call the tenant API (TKT-59HWQ76). Empty turns the API off.
	apiRole string
}

// withheldHostConfigReason is what a tenant's script sees when it reads a
// secret or calls ai.* while the host config is withheld.
const withheldHostConfigReason = "the operator's secrets and AI settings are not shared in tenant mode " +
	"(rela-server -tenant-shared-host-config)"

// envIDPOperatorSecret holds the HMAC secret of the IdP operator API. It is
// read from the environment only, so it never shows in `ps` or shell
// history.
const envIDPOperatorSecret = "RELA_IDP_OPERATOR_SECRET" //nolint:gosec // G101: the name of the variable, not a secret

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
	flag.StringVar(&t.provision, "tenant-provision", envOr("RELA_TENANT_PROVISION", string(tenanthost.ProvisionOff)),
		"Tenant mode: create a tenant for a verified org that has none. off (default): an unknown org is "+
			"refused. webhook: on a verified organization.created IdP webhook (needs -webhook-*). lazy: also "+
			"on the org's first request, which is answered 202 while the tenant is created. Trusts every "+
			"assertion the one configured issuer signs. Also RELA_TENANT_PROVISION.")
	flag.StringVar(&t.apiRole, "tenant-api-role", envOr("RELA_TENANT_API_ROLE", ""),
		"Tenant mode: turn on the tenant API (/api/v1/_tenants/<org_id>) for a control plane. It accepts "+
			"the identity assertion of an operator principal (principal_type operator, no org_id) that "+
			"holds this role, which the edge forwards after verifying the caller. Needs -tenant-provision "+
			"webhook or lazy. Also RELA_TENANT_API_ROLE.")
	flag.IntVar(&t.provisionRate, "tenant-provision-rate", envInt("RELA_TENANT_PROVISION_RATE"),
		"Tenant mode: most tenant creations started per minute (default 10; at most 2 run at once). "+
			"Also RELA_TENANT_PROVISION_RATE.")
	flag.StringVar(&t.idpOrgAPI, "tenant-idp-org-api", envOr("RELA_TENANT_IDP_ORG_API", ""),
		"Tenant mode: the IdP operator API (https://host:port, unix:///path/to/socket, or http://host:port "+
			"for localhost or a loopback IP only). A verified organization.updated webhook suspends or "+
			"resumes the org's tenant to match the IdP, and organization.deleted suspends it. Needs -webhook-* "+
			"and the secret in "+envIDPOperatorSecret+"; required when -webhook-* is set in tenant mode. "+
			"Also RELA_TENANT_IDP_ORG_API.")
	flag.StringVar(&t.luaHTTPAllow, "lua-http-allow", envOr("RELA_LUA_HTTP_ALLOW", ""),
		"Tenant mode: comma-separated CIDRs, IPs and host:port destinations that Lua http.* may reach "+
			"although they are not public addresses, such as an internal IdP. Tenant mode refuses every "+
			"loopback, private and other non-public address otherwise. Also RELA_LUA_HTTP_ALLOW.")
	flag.BoolVar(&t.sharedHostConfig, "tenant-shared-host-config", os.Getenv("RELA_TENANT_SHARED_HOST_CONFIG") == "1",
		"Tenant mode: let every tenant's scripts, flows and connections use .rela/secrets.yaml and "+
			".rela/ai.yaml. These are the operator's own credentials, the same for every tenant, used by "+
			"operator-authored scripts on tenant data. Off by default: secrets and ai.* then fail with an "+
			"error. Mail is always shared. Also RELA_TENANT_SHARED_HOST_CONFIG=1.")
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
	// The provisioning flags are checked by tenanthost.New.
	if _, err := luaHTTPPolicy(f.tenant); err != nil {
		return mode, err
	}
	return mode, nil
}

// tenantProfile is the shared part of tenant mode's SaaS profile, from the
// -lua-http-allow and -tenant-shared-host-config flags.
func tenantProfile(t tenantFlags) appbuild.TenantProfile {
	return appbuild.TenantProfile{
		LuaHTTPAllow:     t.luaHTTPAllow,
		SharedHostConfig: t.sharedHostConfig,
		WithheldReason:   withheldHostConfigReason,
	}
}

// luaHTTPPolicy is tenant mode's Lua HTTP destination policy: strict, with
// the -lua-http-allow destinations reopened.
func luaHTTPPolicy(t tenantFlags) (lua.HTTPPolicy, error) {
	p, err := tenantProfile(t).LuaHTTPPolicy()
	if err != nil {
		return lua.HTTPPolicy{}, fmt.Errorf("-lua-http-allow: %w", err)
	}
	return p, nil
}

// logTenantProfile logs once at startup what tenant mode turns off or
// scopes (TKT-3RWQ1PK), so an operator sees it before the first tenant
// opens. Each tenant logs what it would have served on its first open.
func logTenantProfile(t tenantFlags, p lua.HTTPPolicy) {
	hostConfig := "withheld"
	if t.sharedHostConfig {
		hostConfig = "shared"
	}
	slog.Info("tenant mode: shell commands, git sync and the scheduler are off; "+
		"Lua http.* reaches public addresses only",
		"lua_http_allow", len(p.Allow)+len(p.AllowHosts), "secrets_and_ai", hostConfig, "mail", "shared")
}

// noteTenantRestrictions turns off git sync on a tenant's app and logs,
// once per process, how many configured commands and whether git sync
// tenant mode turned off. Every tenant shares one configuration, so one
// line describes them all.
func (s *server) noteTenantRestrictions(app *dataentry.App) {
	gitWasOn := dataentry.DisableGitSync(app)
	s.tenantNotice.Do(func() {
		if n := len(app.Cfg().Commands); n > 0 || gitWasOn {
			slog.Warn("tenant mode: data-entry.yaml features are turned off",
				"commands", n, "git_sync", gitWasOn)
		}
	})
}

// refuseTenantOnlyFlags refuses, outside tenant mode, a flag that only
// tenant mode acts on, so a server never runs without a behavior its
// operator asked for.
//
// -lua-http-allow is refused too: outside tenant mode Lua http.* is not
// strict, so an allowlist would suggest a restriction that is not there.
func refuseTenantOnlyFlags(t tenantFlags) error {
	if t.enabled {
		return nil
	}
	switch {
	case t.idpOrgAPI != "":
		return errors.New("-tenant-idp-org-api needs tenant mode (-tenant-mode)")
	case t.luaHTTPAllow != "":
		return errors.New("-lua-http-allow needs tenant mode (-tenant-mode): " +
			"outside it Lua http.* may reach every address but link-local and metadata ones")
	case t.sharedHostConfig:
		return errors.New("-tenant-shared-host-config needs tenant mode (-tenant-mode)")
	case t.provision != "" && t.provision != string(tenanthost.ProvisionOff):
		return errors.New("-tenant-provision needs tenant mode (-tenant-mode)")
	case t.provisionRate != 0:
		return errors.New("-tenant-provision-rate needs tenant mode (-tenant-mode)")
	case t.apiRole != "":
		return errors.New("-tenant-api-role needs tenant mode (-tenant-mode)")
	}
	return nil
}

// validateTenantInbound refuses an IdP status-sync or tenant API
// configuration the host cannot act on. secret is
// $RELA_IDP_OPERATOR_SECRET.
func validateTenantInbound(f *serverFlags, secret string) error {
	if err := validateStatusSync(f, secret); err != nil {
		return err
	}
	return validateTenantAPI(f)
}

// apiRolePattern constrains -tenant-api-role to the role names the edge
// can put in an assertion.
var apiRolePattern = regexp.MustCompile(`^[a-z][a-z0-9:_-]{0,63}$`)

// validateTenantAPI refuses a tenant API configuration the host cannot
// serve. The API creates tenants through the provisioning runner, so
// provisioning must be on.
func validateTenantAPI(f *serverFlags) error {
	role := f.tenant.apiRole
	switch {
	case role == "":
		return nil
	case !apiRolePattern.MatchString(role):
		return fmt.Errorf("-tenant-api-role %q must match %s", role, apiRolePattern)
	case f.tenant.provision == "" || f.tenant.provision == string(tenanthost.ProvisionOff):
		return errors.New("-tenant-api-role needs -tenant-provision webhook or lazy")
	}
	return nil
}

// validateStatusSync refuses a status-sync configuration the host cannot
// act on. secret is $RELA_IDP_OPERATOR_SECRET.
//
// With the IdP webhook on, status sync is required: an
// `organization.updated` or `organization.deleted` the host cannot apply
// would leave a deactivated org's users their access, and the host refuses
// such events with 503 rather than acknowledge them. Starting without it
// would turn every deactivation into a retry loop.
func validateStatusSync(f *serverFlags, secret string) error {
	if f.tenant.idpOrgAPI == "" {
		if f.webhookAudience != "" || f.webhookAction != "" {
			return errors.New("tenant mode with the IdP webhook (-webhook-audience, -webhook-action) needs " +
				"-tenant-idp-org-api: an organization suspension or deletion must revoke the tenant's access")
		}
		return nil
	}
	if f.webhookAudience == "" || f.webhookAction == "" {
		return errors.New("-tenant-idp-org-api needs the IdP webhook (-webhook-audience, -webhook-action)")
	}
	if secret == "" {
		return errors.New("-tenant-idp-org-api needs the operator API secret in $" + envIDPOperatorSecret)
	}
	if _, err := idporg.New(f.tenant.idpOrgAPI, []byte(secret)); err != nil {
		return fmt.Errorf("-tenant-idp-org-api: %w", err)
	}
	return nil
}

// idpOrgAdapter is the host's [tenanthost.OrgStatusSource] over the IdP
// operator API client.
type idpOrgAdapter struct{ c *idporg.Client }

// OrgSuspended reports whether orgID is suspended at the IdP.
func (a idpOrgAdapter) OrgSuspended(ctx context.Context, orgID string) (bool, error) {
	st, err := a.c.OrgStatus(ctx, orgID)
	switch {
	case errors.Is(err, idporg.ErrNotFound):
		return false, fmt.Errorf("%w: %w", tenanthost.ErrOrgNotFound, err)
	case errors.Is(err, idporg.ErrInvalidOrgID):
		return false, fmt.Errorf("%w: %w", tenanthost.ErrInvalidOrgID, err)
	}
	if err != nil {
		return false, err
	}
	return st == idporg.StatusSuspended, nil
}

// statusSyncSource builds the IdP side of status sync from the flags, which
// validateStatusSync checked. The caller checked that -tenant-idp-org-api
// is set.
func statusSyncSource(t tenantFlags, secret string) (tenanthost.OrgStatusSource, error) {
	c, err := idporg.New(t.idpOrgAPI, []byte(secret))
	if err != nil {
		return nil, err
	}
	return idpOrgAdapter{c}, nil
}

// provisionAdapter is the host's [tenanthost.Provisioner] over the control
// table's writer. It never adopts an existing schema: a schema without a
// control row may hold another org's data, so only an operator adopts it.
// It never recreates an erased tenant either: an erasure is undone only by
// an operator's `tenant create -recreate`.
//
// A create that made the tenant active is written to the lifecycle log
// with actor [actorProvision].
type provisionAdapter struct {
	p   tenantCreator
	log lifecycleRecorder
}

// tenantCreator is what provisionAdapter needs from [tenant.Provisioner].
type tenantCreator interface {
	Create(ctx context.Context, orgID string, opts tenant.CreateOptions) (tenant.Tenant, error)
}

// Provision creates orgID's tenant. An active tenant that already exists
// is success: two servers may race the same create. One in another status
// is returned as the [tenant.ExistsError]. A refusal only an operator can
// resolve is wrapped in [tenanthost.ErrNeedsOperator].
func (a provisionAdapter) Provision(ctx context.Context, orgID string) error {
	t, err := a.p.Create(ctx, orgID, tenant.CreateOptions{})
	var exists *tenant.ExistsError
	switch {
	case err == nil:
		a.log.recordSystem(actorProvision, "create", orgID, t.Schema)
		return nil
	case errors.As(err, &exists) && exists.Status == tenant.StatusActive:
		// Another server created it and recorded that.
		return nil
	case errors.Is(err, tenant.ErrSchemaExists), errors.Is(err, tenant.ErrSchemaCollision),
		errors.Is(err, tenant.ErrTenantErased), errors.Is(err, tenant.ErrWrongStatus):
		return fmt.Errorf("%w: %w", tenanthost.ErrNeedsOperator, err)
	}
	return err
}

// provisionConfig builds the host's provisioning config from the flags.
// tenanthost.New validates it. p is the server's one provisioner, nil when
// provisioning is off. log records the creates.
func provisionConfig(t tenantFlags, p tenantCreator, log lifecycleRecorder) (tenanthost.ProvisionConfig, error) {
	mode, err := tenanthost.ParseProvisionMode(t.provision)
	if err != nil {
		return tenanthost.ProvisionConfig{}, fmt.Errorf("-tenant-provision: %w", err)
	}
	if mode == tenanthost.ProvisionOff {
		return tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionOff}, nil
	}
	cfg := tenanthost.ProvisionConfig{Mode: mode, PerMinute: t.provisionRate}
	if p != nil {
		cfg.Provisioner = provisionAdapter{p: p, log: log}
	}
	return cfg, nil
}

// tenantWriter builds the server's one [tenant.Provisioner], which serves
// both provisioning and status sync, and the provisioning config over it.
// The provisioner is nil when neither is on. log records the creates.
func tenantWriter(t tenantFlags, baseDSN, auditRoot string, log lifecycleRecorder,
) (tenanthost.ProvisionConfig, *tenant.Provisioner, error) {
	var writer *tenant.Provisioner
	var creator tenantCreator // stays a nil interface without a writer
	if (t.provision != "" && t.provision != string(tenanthost.ProvisionOff)) || t.idpOrgAPI != "" {
		p, err := tenant.NewProvisioner(baseDSN, auditRoot)
		if err != nil {
			return tenanthost.ProvisionConfig{}, nil, err
		}
		writer, creator = p, p
	}
	prov, err := provisionConfig(t, creator, log)
	return prov, writer, err
}

// The actors the server's own lifecycle changes are recorded under. They
// take the place of the operator name the tenant command records.
const (
	actorProvision = "system:provision"
	actorIDP       = "system:idp"
	actorTenantAPI = "system:tenant-api"
)

// lifecycleRecorder writes the server's own lifecycle changes to the
// lifecycle log, the record `rela-server tenant create/suspend/resume`
// writes. The zero value records nothing; tests that do not read the log
// use it.
type lifecycleRecorder struct {
	// cacheDir is the project's .rela directory; the log is in its audit
	// directory, beside the per-tenant ones.
	cacheDir string
}

// recordSystem appends one applied change. It is called after the change
// is committed, so a failure to write the record is logged as an error and
// does not undo or fail the change.
func (l lifecycleRecorder) recordSystem(actor, action, orgID, schema string) {
	l.recordFor(actor, "", action, orgID, schema)
}

// recordFor is recordSystem for a change a caller asked for. principal,
// when set, is that caller's subject, such as the tenant API's operator
// client.
func (l lifecycleRecorder) recordFor(actor, principal, action, orgID, schema string) {
	if l.cacheDir == "" {
		return
	}
	rec := lifecycleEntry(actor, "rela-server", action, orgID, schema, nil)
	if principal != "" {
		rec["principal"] = principal
	}
	if err := writeLifecycle(l.cacheDir, rec); err != nil {
		slog.Error("tenant lifecycle change not recorded", "file", lifecycleLog, "actor", actor,
			"action", action, "org_id", orgID, "error", err)
	}
}

// idpSuspensionAdapter is the host's [tenanthost.IDPSuspension] over the
// control table's writer. Each change it applies is written to the
// lifecycle log with actor [actorIDP].
type idpSuspensionAdapter struct {
	w   tenanthost.IDPSuspension
	log lifecycleRecorder
}

// SuspendForIDP suspends orgID's tenant and records the change.
func (a idpSuspensionAdapter) SuspendForIDP(ctx context.Context, orgID string) (bool, error) {
	changed, err := a.w.SuspendForIDP(ctx, orgID)
	if err == nil && changed {
		a.log.recordSystem(actorIDP, "suspend", orgID, "")
	}
	return changed, err
}

// ResumeForIDP resumes orgID's tenant and records the change.
func (a idpSuspensionAdapter) ResumeForIDP(ctx context.Context, orgID string) (bool, error) {
	changed, err := a.w.ResumeForIDP(ctx, orgID)
	if err == nil && changed {
		a.log.recordSystem(actorIDP, "resume", orgID, "")
	}
	return changed, err
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
	// Before StartWatching, which would start git sync's periodic fetch.
	s.noteTenantRestrictions(app)
	if err := app.StartWatching(); err != nil {
		slog.Warn("tenant: file watcher not started", "org_id", t.OrgID, "schema", t.Schema, "error", err)
	}
	handler := app.NewRouter(dataentry.WithAccessLog(s.accessLog), dataentry.WithQueryCeiling(s.f.queryCeiling))

	// The scheduler is not started per tenant (TKT-M3SHYE0). The workers
	// below serve live features of the app and read only this tenant's
	// services. The flow worker runs while the tenant is resident, as the
	// realtime timers do: an evicted tenant's due runs wait until it opens
	// again (TKT-FWQZGAM).
	ctx, cancel := context.WithCancel(context.Background())
	leases := appbuild.WatchLeases(ctx, svc)
	worker := appbuild.RunRealtimeWorker(ctx, svc)
	flows := appbuild.RunFlowWorker(ctx, svc)
	follower := appbuild.FollowChannels(ctx, svc)
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-leases
		<-worker
		<-flows
		<-follower
	}()
	slog.Info("tenant opened", "org_id", t.OrgID, "schema", t.Schema)
	return &tenantResident{
		t: t, opened: opened, app: app, handler: handler, relay: relay, stop: cancel, done: done,
	}, nil
}

// tenantOptions are the registry bounds, where zero takes the registry's
// defaults, the host's provisioning config, and the two sides of status
// sync. Status sync is on when both orgStatus and idpWriter are set.
type tenantOptions struct {
	resident, open int
	provision      tenanthost.ProvisionConfig
	orgStatus      tenanthost.OrgStatusSource
	idpWriter      tenanthost.IDPSuspension
	tenantAPI      *tenantAPIOptions
}

// tenantHost is a running multi-tenant server: the front handler and the
// registry it leases tenants from.
type tenantHost struct {
	addr     string
	handler  http.Handler
	host     *tenanthost.Host
	registry *tenant.Registry[*tenantResident]
	// resolver is closed after the registry.
	resolver io.Closer
	// sweeper opens evicted tenants with due background work.
	sweeper *tenant.Sweeper
}

// close ends the host's provisioning runs, then closes the registry and
// the resolver it reads.
func (th *tenantHost) close() {
	th.host.Close()
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
	secret := os.Getenv(envIDPOperatorSecret)
	if verr := validateTenantInbound(f, secret); verr != nil {
		return nil, verr
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
	// The strict Lua HTTP policy is set before anything can build a Lua
	// runtime; validateTenantMode has parsed the allowlist. Startup fails
	// unless the strict policy is installed afterwards.
	policy, err := tenantProfile(f.tenant).ApplyLuaPolicy()
	if err != nil {
		return nil, err
	}
	logTenantProfile(f.tenant, policy)
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

	lifecycle := lifecycleRecorder{cacheDir: base.CacheDir()}
	prov, writer, err := tenantWriter(f.tenant, os.Getenv("RELA_DATABASE_URL"), tenant.AuditRoot(base.CacheDir()),
		lifecycle)
	if err != nil {
		_ = resolver.Close()
		return nil, err
	}
	opts := tenantOptions{resident: f.tenant.maxResident, open: f.tenant.maxOpen, provision: prov,
		tenantAPI: newTenantAPIOptions(f.tenant.apiRole, resolver, writer, base.UserEntityType(), lifecycle)}
	if f.tenant.idpOrgAPI != "" {
		if opts.orgStatus, err = statusSyncSource(f.tenant, secret); err != nil {
			_ = resolver.Close()
			return nil, err
		}
		opts.idpWriter = idpSuspensionAdapter{w: writer, log: lifecycle}
	}
	th, err := s.tenantHost(base, resolver, opts, adapter)
	if err != nil {
		_ = resolver.Close()
		return nil, err
	}
	th.resolver = resolver
	if th.sweeper, err = tenant.NewSweeper(resolver, th.registry); err != nil {
		th.close()
		return nil, err
	}
	return th, nil
}

// shellBoot reads the app name and language the tenant-free shell shows
// from the shared data-entry.yaml. An unreadable file leaves the shell as
// built: the tenants' own config load reports a broken file.
func shellBoot(base *appbuild.SharedBase) dataentry.ShellBoot {
	boot, err := dataentry.LoadShellBoot(context.Background(), base.ProjectConfig().Load)
	if err != nil {
		slog.Warn("tenant mode: shell reads no app name", "error", err)
		return dataentry.ShellBoot{}
	}
	return boot
}

// tenantHost wires the registry and the front handler over one shared base.
// verifier is the host's own; s.assertion must already be the reuse wrapper
// around the same verifier, so every tenant's gate trusts what the host
// verified and nothing else.
func (s *server) tenantHost(
	base *appbuild.SharedBase, resolver tenant.Resolver, limits tenantOptions, verifier tenanthost.AssertionVerifier,
) (*tenantHost, error) {
	openSvc, err := tenant.SharedBaseOpener(base)
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
		Static:     dataentry.StaticShell(shellBoot(base)),
		Provision:  limits.provision,
		TenantAPI:  limits.tenantAPI.config(registry.RevalidateOrg),
		Acquire: func(ctx context.Context, orgID string) (http.Handler, func(), error) {
			lease, aerr := registry.Acquire(ctx, orgID)
			if aerr != nil {
				return nil, nil, aerr
			}
			if lease.Tenant.ReadOnly {
				return tenanthost.ReadOnly(lease.Value().handler), lease.Release, nil
			}
			return lease.Value().handler, lease.Release, nil
		},
	}
	if s.webhook != nil {
		hcfg.Webhook = webhookVerifierAdapter{s.webhook}
	}
	if limits.orgStatus != nil && limits.idpWriter != nil {
		if hcfg.StatusSync, err = tenanthost.NewStatusSync(limits.orgStatus, limits.idpWriter,
			registry.RevalidateOrg); err != nil {
			_ = registry.Close()
			return nil, err
		}
	}
	host, err := tenanthost.New(hcfg)
	if err != nil {
		_ = registry.Close()
		return nil, err
	}
	mode := limits.provision.Mode
	if mode == "" {
		mode = tenanthost.ProvisionOff
	}
	slog.Info("tenant mode enabled", "max_resident", limits.resident, "max_open", limits.open,
		"provision", string(mode), "idp_status_sync", hcfg.StatusSync != nil, "tenant_api", hcfg.TenantAPI != nil)
	return &tenantHost{addr: s.addr, handler: host, host: host, registry: registry}, nil
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
	// Open evicted tenants whose timers or jobs are due, so they run without
	// a request (TKT-ZOU5BCT).
	swept := make(chan struct{})
	go func() {
		defer close(swept)
		th.sweeper.Run(ctx, tenant.SweepInterval)
	}()
	httpSrv := newHTTPServer(th.addr, th.handler)
	// End every tenant's streams when shutdown starts, so the drain does not
	// wait out the grace for each open tab (TKT-N3X235N).
	httpSrv.RegisterOnShutdown(th.registry.RevokeAll)
	serveErr := serveUntilSignal(httpSrv)
	stop()
	// Wait for a revalidation or sweep pass in progress before the registry
	// and the resolver it uses close.
	<-revalidated
	<-swept
	th.close()
	if serveErr != nil {
		slog.Error("server stopped", "error", serveErr)
		return 1
	}
	return 0
}

// coverage-ignore-end
