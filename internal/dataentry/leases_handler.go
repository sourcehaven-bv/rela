package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const (
	leasesPath = "/api/v1/_leases"
	// leaseBodyLimit caps a lease request body: a lease id, a label and a
	// status of 140 characters fit many times over.
	leaseBodyLimit = 4 << 10
	// leaseFullRetryAfter is the Retry-After of a 503 for a full lease store,
	// in seconds.
	leaseFullRetryAfter = "30"
)

// LeaseService is the advisory lease capability the lease routes and the
// `_lease` field use. The wiring site passes the project's *lease.Service,
// which applies the read and update checks itself; see that type for the
// access rules.
type LeaseService interface {
	Claim(ctx context.Context, req lease.ClaimRequest) (lease.Claimed, error)
	Renew(ctx context.Context, leaseID string, ttl time.Duration, status *string) (lease.Lease, error)
	Release(ctx context.Context, leaseID string) error
	// GetReadable returns the live leases on ids. It applies no access
	// check: pass only ids of rows the caller has already read.
	GetReadable(ctx context.Context, ids []string) (map[string]lease.Lease, error)
	Mine(ctx context.Context) ([]lease.Lease, error)
	Subscribe() (wake <-chan struct{}, cancel func())
}

// leasesHandler owns the lease routes (TKT-4YPG4Q):
//
//	POST /api/v1/{plural}/{id}/_lease   claim (201)
//	POST /api/v1/_leases/renew          renew (200)
//	POST /api/v1/_leases/release        release (204, always)
//	GET  /api/v1/_leases                the caller's leases (200)
//
// It also fills `_lease` on the single-entity GET, on list rows and on the
// entry of a view.
//
// A nil *leasesHandler is "leases not wired": the claim route answers the
// entity GET's uniform 404, the other routes a plain 404, and no response
// carries `_lease`. [SetLeases] installs it.
//
// The lease id is a secret: it travels only in a claim answer and in renew
// and release bodies, never in a URL, a log line or another response.
type leasesHandler struct {
	svc LeaseService
}

// SetLeases installs the advisory lease service. Call it before
// [App.StartWatching] and [App.NewRouter]: the live worker takes its wake-up
// source when it starts, and the router binds the lease routes when it is
// built. It is a package function rather than an App method because App is
// at its plimsoll method cap.
//
// Nil: rejected — a deployment without leases simply never calls this, so a
// nil here is a wiring bug. The parameter is the concrete pointer so that a
// nil *lease.Service is caught here rather than stored as a non-nil
// interface holding a nil pointer.
func SetLeases(a *App, svc *lease.Service) error {
	if svc == nil {
		return errors.New("dataentry: SetLeases requires a lease service")
	}
	a.leases = &leasesHandler{svc: svc}
	a.views.leases = a.leases
	return nil
}

// wakeSource returns the lease wake-ups the live worker listens to, or nil
// when leases are not wired.
func (h *leasesHandler) wakeSource() leaseWaker {
	if h == nil {
		return nil
	}
	return h.svc
}

// handleClaim serves POST /api/v1/{plural}/{id}/_lease. typeName is the
// type the route's plural names; an entity of another type is not found.
//
// Every miss (hidden, nonexistent, another type, leases not wired) is the
// entity GET's uniform 404, and a bare id of a faced type is the write
// path's 422 face_required. A caller who may read but not update gets a 403
// that names no holder: the service asks about update access before it
// looks at the lease.
//
//archplan:auth=authz
func (h *leasesHandler) handleClaim(w http.ResponseWriter, r *http.Request, typeName, addr string) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	var req v1.LeaseClaimRequest
	if !decodeLeaseBody(w, r, &req, true) {
		return
	}
	ttl, ok := leaseTTL(w, r, req.TTLSeconds)
	if !ok {
		return
	}
	c, err := h.svc.Claim(r.Context(), lease.ClaimRequest{
		Addr: addr, TTL: ttl, Label: req.Label, Status: req.Status, Type: typeName,
	})
	if err != nil {
		writeClaimError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusCreated, v1.LeaseClaimed{
		LeaseID: c.LeaseID, Token: c.Lease.Token, ExpiresAt: leaseTime(c.Lease.Expires),
	})
}

// handleV1Leases serves every route under /api/v1/_leases.
//
//archplan:auth=required
func (h *leasesHandler) handleV1Leases(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	switch strings.Trim(strings.TrimPrefix(r.URL.Path, leasesPath), "/") {
	case "":
		if allowMethod(w, r, http.MethodGet) {
			h.listMine(w, r)
		}
	case "renew":
		if allowMethod(w, r, http.MethodPost) {
			h.renew(w, r)
		}
	case "release":
		if allowMethod(w, r, http.MethodPost) {
			h.release(w, r)
		}
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	}
}

func (h *leasesHandler) renew(w http.ResponseWriter, r *http.Request) {
	var req v1.LeaseRenewRequest
	if !decodeLeaseBody(w, r, &req, false) {
		return
	}
	ttl, ok := leaseTTL(w, r, req.TTLSeconds)
	if !ok {
		return
	}
	l, err := h.svc.Renew(r.Context(), req.LeaseID, ttl, req.Status)
	if err != nil {
		writeLeaseError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, v1.LeaseRenewed{Token: l.Token, ExpiresAt: leaseTime(l.Expires), Status: l.Status})
}

// release answers 204 whether or not the caller held the lease, so the
// answer says nothing about other people's leases. A lease id that is not
// the shape a claim returns is a 400, as it is over MCP.
func (h *leasesHandler) release(w http.ResponseWriter, r *http.Request) {
	var req v1.LeaseReleaseRequest
	if !decodeLeaseBody(w, r, &req, false) {
		return
	}
	if err := h.svc.Release(r.Context(), req.LeaseID); err != nil {
		writeLeaseError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listMine answers GET /_leases: the caller's live leases on entities the
// caller may read now, without lease ids.
func (h *leasesHandler) listMine(w http.ResponseWriter, r *http.Request) {
	mine, err := h.svc.Mine(r.Context())
	if err != nil {
		writeLeaseError(w, r, err)
		return
	}
	out := v1.OwnLeaseList{Data: make([]v1.OwnLease, 0, len(mine))}
	for _, l := range mine {
		out.Data = append(out.Data, v1.OwnLease{
			ID: l.ID, Since: leaseTime(l.Since), ExpiresAt: leaseTime(l.Expires),
			Status: l.Status, Label: l.Holder.Label,
		})
	}
	writeV1JSON(w, http.StatusOK, out)
}

// entityLease returns the `_lease` of the entity with id, or nil when it is
// not held or leases are not wired. id must be the id of an entity the
// caller has just read through the row gate. A failed read is logged and
// leaves the field absent: the lease decorates the entity, so it must not
// fail the read.
func (h *leasesHandler) entityLease(ctx context.Context, id string) *v1.EntityLease {
	if h == nil {
		return nil
	}
	got, err := h.svc.GetReadable(ctx, []string{id})
	if err != nil {
		slog.WarnContext(ctx, "dataentry: reading the lease failed; _lease omitted", "entity", id, "err", err)
		return nil
	}
	return wireLease(got, id)
}

// serveRowLeases sets `_lease` on each row in data, which is parallel to
// rows. The rows have passed the read gate. It costs one lease read for the
// whole page. A failed read is logged and leaves every `_lease` absent.
func (h *leasesHandler) serveRowLeases(ctx context.Context, rows []*entityPkg.Entity, data []v1.Entity) {
	if h == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, e := range rows {
		if !seen[e.ID] {
			seen[e.ID] = true
			ids = append(ids, e.ID)
		}
	}
	got, err := h.svc.GetReadable(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "dataentry: reading leases failed; _lease omitted", "err", err)
		return
	}
	for i, e := range rows {
		data[i].Lease = wireLease(got, e.ID)
	}
}

// wireLease is the `_lease` of id in got, or nil.
func wireLease(got map[string]lease.Lease, id string) *v1.EntityLease {
	l, ok := got[id]
	if !ok {
		return nil
	}
	return &v1.EntityLease{
		User: l.Holder.User, Tool: l.Holder.Tool, Label: l.Holder.Label, Since: leaseTime(l.Since), Status: l.Status,
	}
}

func leaseTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// leaseTTL converts ttl_seconds to a duration: zero for the default, a 400
// outside [lease.MinTTL, lease.MaxTTL]. The range is checked before the
// conversion, which overflows for a huge value.
func leaseTTL(w http.ResponseWriter, r *http.Request, seconds int) (time.Duration, bool) {
	if seconds == 0 {
		return 0, true
	}
	lo, hi := int(lease.MinTTL/time.Second), int(lease.MaxTTL/time.Second)
	if seconds < lo || seconds > hi {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid lease request",
			"ttl_seconds must be between "+strconv.Itoa(lo)+" and "+strconv.Itoa(hi))
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

// decodeLeaseBody reads a JSON body of at most [leaseBodyLimit] bytes into
// dst. An empty body is accepted only when emptyOK. The decode is strict, as
// on the entity create path: an unknown field is a 400 rather than silently
// ignored, so a misspelled ttl_seconds does not claim for the default
// duration, and data after the JSON value is a 400 too.
func decodeLeaseBody(w http.ResponseWriter, r *http.Request, dst any, emptyOK bool) bool {
	data, err := io.ReadAll(io.LimitReader(r.Body, leaseBodyLimit+1))
	switch {
	case err != nil:
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Could not read the request body", "")
		return false
	case len(data) > leaseBodyLimit:
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "")
		return false
	case len(bytes.TrimSpace(data)) == 0 && emptyOK:
		return true
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", err.Error())
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON",
			"unexpected data after the JSON object")
		return false
	}
	return true
}

// writeClaimError answers a failed claim. A miss and a bare faced address go
// through the write path's resolution errors, so their bodies are the ones
// an entity GET and a PATCH give; a held entity is a 409 naming the holder.
func writeClaimError(w http.ResponseWriter, r *http.Request, err error) {
	var amb *visibility.AmbiguousAddressError
	var held *lease.HeldError
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeWriteTargetError(w, r, errWriteTargetNotFound)
	case errors.As(err, &amb):
		writeWriteTargetError(w, r, amb)
	case errors.Is(err, acl.ErrForbidden):
		if !writeForbiddenIfACLDenied(w, err) {
			writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not allowed to update this entity", "")
		}
	case errors.As(err, &held):
		writeLeaseHeld(w, r, held.Current)
	default:
		writeLeaseError(w, r, err)
	}
}

// writeLeaseHeld answers 409 lease_held with the live lease.
func writeLeaseHeld(w http.ResponseWriter, r *http.Request, cur lease.Lease) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(v1.LeaseHeld{
		Error: v1.Error{
			Type: "https://rela.dev/errors/lease_held", Title: "Entity is held", Status: http.StatusConflict,
			Instance: r.URL.Path,
		},
		Holder: v1.LeaseHolder{User: cur.Holder.User, Tool: cur.Holder.Tool, Label: cur.Holder.Label},
		Since:  leaseTime(cur.Since), ExpiresAt: leaseTime(cur.Expires), Status: cur.Status,
	})
}

// writeLeaseError answers the lease errors every route shares. An
// unexpected error is logged without the request body, which holds the
// lease id, and answered generically.
func writeLeaseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, lease.ErrLost):
		writeV1Error(w, r, http.StatusConflict, "lease_lost", "Lease lost",
			"the lease expired, was released, or you no longer have access to the entity; claim it again")
	case errors.Is(err, lease.ErrLimit):
		writeV1Error(w, r, http.StatusConflict, "lease_limit", "Too many leases",
			"at most "+strconv.Itoa(lease.MaxPerIdentity)+" live leases per user and tool, "+
				"shared by every session and agent running as that user through the same tool")
	case errors.Is(err, lease.ErrFull):
		w.Header().Set("Retry-After", leaseFullRetryAfter)
		writeV1Error(w, r, http.StatusServiceUnavailable, "lease_store_full", "The lease store is full",
			"retry later")
	case errors.Is(err, lease.ErrInvalid):
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid lease request",
			strings.TrimPrefix(err.Error(), lease.ErrInvalid.Error()+": "))
	case errors.Is(err, lease.ErrNoPrincipal):
		writeV1Error(w, r, http.StatusForbidden, "no_principal", "Leases need a signed-in user", "")
	default:
		writeInternalError(w, r, "lease_failed", "Lease request failed", err)
	}
}
