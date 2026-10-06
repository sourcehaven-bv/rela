package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// personGetter is the gated single-entity read the account endpoint needs.
// Satisfied by visibility.Reader: the row gate and field redaction are the
// same ones every other read-out path goes through.
type personGetter interface {
	Get(ctx context.Context, entityType, id string) (*entityPkg.Entity, bool, error)
}

// meDeps is what [buildMe] reads. typeOf is an UNGATED type lookup: it only
// picks the type argument for the gated Get, and its answer reaches the wire
// only when that Get succeeds, so it discloses nothing the Get does not.
type meDeps struct {
	meta     *metamodel.Metamodel
	account  *dataentryconfig.AccountConfig
	people   personGetter
	typeOf   func(ctx context.Context, id string) string
	userType string // acl.yaml user_entity_type; "" when unset or no ACL
}

// handleV1Me serves GET /api/v1/_me: who the request principal is, for the
// SPA's account menu (TKT-MJTD12). See [v1.MeResponse].
//
// A package function taking the App rather than a method: App is at its
// plimsoll load line, and this needs only what it names.
func handleV1Me(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	s := a.State()
	deps := meDeps{
		meta:    s.Meta,
		account: s.Cfg.Account,
		people:  a.viewReader,
		typeOf:  a.reader.entityType,
	}
	if d, ok := a.acl.(*acl.Declarative); ok {
		if pol := d.Policy(); pol != nil {
			deps.userType = pol.UserEntityType
		}
	}
	ctx := r.Context()
	// Per principal; writeV1JSON marks every v1 response no-store.
	me := buildMe(ctx, principal.From(ctx), deps)
	me.CanConfigure = a.configure != nil && mayConfigure(a, r)
	writeV1JSON(w, http.StatusOK, me)
}

// buildMe assembles the account response for p.
func buildMe(ctx context.Context, p principal.Principal, d meDeps) v1.MeResponse {
	out := v1.MeResponse{
		User:  p.User,
		Email: p.Email(),
		Roles: p.Roles(),
	}
	if id := p.OrgID(); id != "" {
		out.Org = &v1.MeOrg{ID: id, Slug: p.OrgSlug(), Name: p.OrgName()}
	}
	out.Person = mePerson(ctx, p.User, d)
	out.Links = meLinks(d.account, p.Roles())
	return out
}

// mePerson returns the entity the principal resolves to, read through the
// gated reader, or nil when there is none or it is hidden. A principal user
// id is an entity id when acl.yaml's principal_property matched one, or when
// the identity itself names an entity (RELA_DATAENTRY_USER=PERS-...).
//
// When acl.yaml declares a user_entity_type, only an entity of that type is a
// person: a subject that happens to equal some other entity's id is not.
func mePerson(ctx context.Context, userID string, d meDeps) *v1.MePerson {
	if userID == "" || d.people == nil || d.typeOf == nil {
		return nil
	}
	typ := d.typeOf(ctx, userID)
	if typ == "" || (d.userType != "" && typ != d.userType) {
		return nil
	}
	e, ok, err := d.people.Get(ctx, typ, userID)
	if err != nil {
		// The menu still works without the person; a gate fault is logged
		// and the person omitted, never served ungated.
		slog.WarnContext(ctx, "dataentry: _me: person read failed", "err", err)
		return nil
	}
	if !ok {
		return nil
	}
	person := &v1.MePerson{Type: e.Type, ID: e.ID, Title: e.ID}
	if d.meta != nil {
		person.Title = safeDisplayTitle(d.meta, e)
		if d.account != nil {
			person.Avatar = avatarURL(d.meta, e, d.account.AvatarProperty)
		}
	}
	return person
}

// avatarURL derives an <img src> from the (already redacted) person entity's
// avatar property. A `file` property yields the attachment download URL the
// entity GET advertises in `_attachments`, so the image is served through the
// same ACL-gated endpoint. A `string` property is passed through only when it
// is a root-relative path or an https URL; anything else is dropped.
func avatarURL(meta *metamodel.Metamodel, e *entityPkg.Entity, prop string) string {
	if prop == "" {
		return ""
	}
	def, ok := meta.GetEntityDef(e.Type)
	if !ok {
		return ""
	}
	pd, ok := def.Properties[prop]
	if !ok {
		return ""
	}
	switch pd.Type {
	case metamodel.PropertyTypeFile:
		file := firstString(e.Properties[prop])
		if file == "" {
			return ""
		}
		// The stored value is the attachment path
		// (attachments/<id>/<prop>/<file>); the endpoint takes the file name.
		return "/api/v1/" + def.GetPlural(e.Type) + "/" + url.PathEscape(e.ID) +
			"/_attachments/" + url.PathEscape(prop) + "/" + url.PathEscape(path.Base(file))
	case metamodel.PropertyTypeString:
		v := firstString(e.Properties[prop])
		if v != "" && dataentryconfig.ValidAccountLink(v) {
			return v
		}
	}
	return ""
}

// firstString returns v when it is a non-empty string, or the first non-empty
// string of a list value, else "".
func firstString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []string:
		for _, s := range t {
			if s != "" {
				return s
			}
		}
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// meLinks projects the configured account links. Each is re-checked with the
// load-time rule because a live config reload does not re-run validation; an
// invalid link is dropped rather than served.
//
// The admin link's role check is a display filter on the ASSERTED roles, not
// an access decision (see [dataentryconfig.AccountAdminLink]).
func meLinks(acc *dataentryconfig.AccountConfig, roles []string) *v1.MeLinks {
	if acc == nil {
		return nil
	}
	valid := func(s string) string {
		if s != "" && dataentryconfig.ValidAccountLink(s) {
			return s
		}
		return ""
	}
	links := v1.MeLinks{
		SignOut:   valid(acc.SignOut),
		Account:   valid(acc.Account),
		SwitchOrg: valid(acc.SwitchOrg),
	}
	if adm := acc.Admin; adm != nil && (adm.Role == "" || slices.Contains(roles, adm.Role)) {
		links.Admin = valid(adm.URL)
	}
	if links == (v1.MeLinks{}) {
		return nil
	}
	return &links
}

// checkAvatarUserType completes the load-time check of
// `account.avatar_property`. dataentryconfig can only check that SOME entity
// type declares it, because the type principals resolve to lives in
// acl.yaml. When acl.yaml names a user_entity_type, the property must be a
// file or string property of that type. Without one, the person may be of
// any type and the metamodel-wide check is all there is.
func checkAvatarUserType(acc *dataentryconfig.AccountConfig, aclImpl acl.ACL, meta *metamodel.Metamodel) error {
	if acc == nil || acc.AvatarProperty == "" || meta == nil {
		return nil
	}
	d, ok := aclImpl.(*acl.Declarative)
	if !ok {
		return nil
	}
	pol := d.Policy()
	if pol == nil || pol.UserEntityType == "" {
		return nil
	}
	def, ok := meta.GetEntityDef(pol.UserEntityType)
	if !ok {
		return nil // acl.yaml validation reports an unknown user type
	}
	pd, ok := def.Properties[acc.AvatarProperty]
	if !ok || !dataentryconfig.AvatarPropertyType(pd.Type) {
		return fmt.Errorf("account.avatar_property: %q is not a file or string property of %q "+
			"(acl.yaml user_entity_type)", acc.AvatarProperty, pol.UserEntityType)
	}
	return nil
}
