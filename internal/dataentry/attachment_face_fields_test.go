package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestFacedAttachment_FaceFieldsBlock pins GitHub #1760 part 2 (TKT-RSVJKS):
// a `fields:` block keyed `ticket@published` lets a role attach a file to the
// published face while every other field of that face stays read-only, and
// leaves the draft face fully writable.
func TestFacedAttachment_FaceFieldsBlock(t *testing.T) {
	app, d := facedAttachmentApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"signer": {
					Read:   allTicketFaces,
					Update: []string{"ticket@draft", "ticket@published"},
					Fields: map[string][]acl.FieldGrant{"ticket@published": {{Field: "screenshot"}}},
				},
			},
			Assignments: map[string]string{"sam": "signer"},
		}, st)
	})
	resolver, err := affordances.New(app.State().Meta, storeRelationLookup{st: app.store}, d)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	setTestResolver(t, app, &policyResolver{inner: resolver})
	sam := principalCtx("sam")

	for _, tc := range []struct {
		name string
		do   func() *httptest.ResponseRecorder
		want int
	}{
		{"upload to the named field on the face", func() *httptest.ResponseRecorder {
			return putAttachmentAs(sam, t, app, d, "TKT-1@published", "screenshot", "signed.txt", []byte("pdf"))
		}, http.StatusOK},
		{"other field on the face is read-only", func() *httptest.ResponseRecorder {
			return patchEntityAs(sam, t, app, d, "ticket", "tickets", "TKT-1@published",
				`{"properties":{"title":"changed"}}`, nil)
		}, http.StatusForbidden},
		{"other face stays writable", func() *httptest.ResponseRecorder {
			return patchEntityAs(sam, t, app, d, "ticket", "tickets", "TKT-1@draft",
				`{"properties":{"title":"changed"}}`, nil)
		}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := tc.do(); rec.Code != tc.want {
				t.Errorf("= %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
	if got := attachmentsOf(sam, t, app, d, "TKT-1@published"); len(got) != 1 || got[0] != "signed.txt" {
		t.Errorf("published attachments = %v, want [signed.txt]", got)
	}
}
