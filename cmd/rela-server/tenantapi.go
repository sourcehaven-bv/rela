package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// tenantAPIOptions are the parts of the tenant API (TKT-59HWQ76) that
// exist before the registry does. [tenantAPIOptions.config] completes them
// with the registry's revalidation.
type tenantAPIOptions struct {
	role     string
	reader   tenantAPIReader
	writer   tenantAPIWriter
	userType string
	log      lifecycleRecorder
}

// newTenantAPIOptions returns the tenant API's options, or nil when the API
// is off: no role is set, or the build has no control-table writer.
func newTenantAPIOptions(role string, reader tenantAPIReader, writer *tenant.Provisioner, userType string,
	log lifecycleRecorder,
) *tenantAPIOptions {
	if role == "" || writer == nil {
		return nil
	}
	return &tenantAPIOptions{role: role, reader: reader, writer: writer, userType: userType, log: log}
}

// tenantAPIReader is what the tenant API reads from
// [tenant.ClusterResolver].
type tenantAPIReader interface {
	Lookup(ctx context.Context, orgID string) (tenant.Record, error)
	Usage(ctx context.Context, orgID, userType string, now time.Time) (tenant.Usage, error)
}

// tenantAPIWriter is what the tenant API changes through
// [tenant.Provisioner].
type tenantAPIWriter interface {
	Apply(ctx context.Context, orgID string, c tenant.Change) (tenant.Record, error)
	Archive(ctx context.Context, orgID string) (tenant.Record, bool, error)
}

// config builds the host's tenant API config. revalidate closes an org's
// open tenant when it is no longer served. Nil: accepted — a nil o is the
// API turned off, and config returns nil.
func (o *tenantAPIOptions) config(revalidate func(context.Context, string)) *tenanthost.TenantAPIConfig {
	if o == nil {
		return nil
	}
	return &tenanthost.TenantAPIConfig{Role: o.role, Control: tenantControl{
		reader: o.reader, writer: o.writer, userType: o.userType, log: o.log, revalidate: revalidate,
	}}
}

// tenantControl is the host's [tenanthost.TenantControl]. Each change it
// applies is written to the lifecycle log with actor [actorTenantAPI] and
// the operator principal's subject. A change of state revalidates the org
// at once, so a suspended or archived tenant is closed now rather than at
// the next revalidation pass.
type tenantControl struct {
	reader     tenantAPIReader
	writer     tenantAPIWriter
	userType   string
	log        lifecycleRecorder
	revalidate func(context.Context, string)
}

func (c tenantControl) Lookup(ctx context.Context, orgID string) (tenant.Record, error) {
	return c.reader.Lookup(ctx, orgID)
}

func (c tenantControl) Apply(ctx context.Context, orgID, actor string, ch tenant.Change) (tenant.Record, error) {
	rec, err := c.writer.Apply(ctx, orgID, ch)
	if err != nil {
		return tenant.Record{}, err
	}
	if ch.State != nil {
		c.log.recordFor(actorTenantAPI, actor, "state "+string(*ch.State), orgID, rec.Schema)
		c.revalidate(ctx, orgID)
	}
	if ch.Plan != nil || ch.Entitlements != nil {
		c.log.recordFor(actorTenantAPI, actor, "plan", orgID, rec.Schema)
	}
	slog.InfoContext(ctx, "tenant api: tenant changed", "org_id", orgID, "actor", actor,
		"state_set", ch.State != nil, "plan_set", ch.Plan != nil, "entitlements_set", ch.Entitlements != nil)
	return rec, nil
}

func (c tenantControl) Archive(ctx context.Context, orgID, actor string) (tenant.Record, error) {
	rec, changed, err := c.writer.Archive(ctx, orgID)
	if err != nil {
		return tenant.Record{}, err
	}
	if changed {
		c.log.recordFor(actorTenantAPI, actor, "archive", orgID, rec.Schema)
		slog.InfoContext(ctx, "tenant api: tenant archived", "org_id", orgID, "actor", actor)
	}
	c.revalidate(ctx, orgID)
	return rec, nil
}

func (c tenantControl) Usage(ctx context.Context, orgID string) (tenant.Usage, error) {
	return c.reader.Usage(ctx, orgID, c.userType, time.Now())
}
