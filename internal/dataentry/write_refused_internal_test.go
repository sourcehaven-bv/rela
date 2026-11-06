package dataentry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestWriteWriteRefused_FieldErrors pins that a validation refusal lists
// each failure in errors[], at the property it names, so the SPA can show a
// strict-mode 422 at the field (DEC-OYV2AK).
func TestWriteWriteRefused_FieldErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []v1.FieldError
	}{
		{
			name: "property and rule failures",
			err: fmt.Errorf("wrapped: %w", &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{
				{Type: metamodel.ValidationErrorRequired, Property: "naam", Message: "naam is required"},
				{Type: metamodel.ValidationErrorInvalidValue, Property: "a/b~c", Message: "not an allowed value"},
				{Type: metamodel.ValidationErrorRule, Message: `validation rule "r" is not satisfied`},
			}}),
			want: []v1.FieldError{
				{Source: v1.ErrorSource{Pointer: "/properties/naam"}, Code: "required", Detail: "naam is required"},
				{Source: v1.ErrorSource{Pointer: "/properties/a~1b~0c"}, Code: "invalid_value",
					Detail: "not an allowed value"},
				{Source: v1.ErrorSource{Pointer: ""}, Code: "rule", Detail: `validation rule "r" is not satisfied`},
			},
		},
		{
			name: "not a validation error",
			err:  errors.New("something else"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeWriteRefused(rec, httptest.NewRequest(http.MethodPost, "/api/v1/verwerkingen", http.NoBody), tc.err)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
			var got v1.Error
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Type != "https://rela.dev/errors/validation_failed" {
				t.Errorf("type = %q", got.Type)
			}
			if len(got.Errors) != len(tc.want) {
				t.Fatalf("errors = %+v, want %+v", got.Errors, tc.want)
			}
			for i := range tc.want {
				if got.Errors[i] != tc.want[i] {
					t.Errorf("errors[%d] = %+v, want %+v", i, got.Errors[i], tc.want[i])
				}
			}
		})
	}
}
