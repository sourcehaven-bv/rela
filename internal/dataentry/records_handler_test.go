package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// recordOwnersStub reads every owner but HIDDEN and updates every owner but
// LOCKED; a known owner's type comes from its prefix.
type recordOwnersStub struct{}

func (recordOwnersStub) Readable(_ context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = id != "HIDDEN"
	}
	return out, nil
}

func (recordOwnersStub) Writable(_ context.Context, id string) (records.Owner, error) {
	switch {
	case id == "HIDDEN" || !strings.HasPrefix(id, "ACC-") && id != "LOCKED":
		return records.Owner{}, records.ErrNotFound
	case id == "LOCKED":
		return records.Owner{}, records.ErrForbidden
	}
	return records.Owner{ID: id, Type: "account"}, nil
}

func newRecordsHandlerT(t *testing.T) *recordsHandler {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`version: "1"
entities:
  account:
    label: Account
    id_prefix: "ACC-"
    properties:
      name: {type: string}
records:
  posting:
    owner: account
    ledger: {amount: amount, group_by: txn}
    fields:
      amount: {type: decimal, scale: 2, required: true}
      txn: {type: string, required: true}
`))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	svc, err := records.NewService(records.Deps{Store: st, Owners: recordOwnersStub{}, Meta: meta})
	require.NoError(t, err)
	return &recordsHandler{svc: svc}
}

func serveRecords(h *recordsHandler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handleV1Records(rec, req)
	return rec
}

const balancedT1 = `{"rows":[` +
	`{"owner":"ACC-1","ts":"2026-03-01T00:00:00Z","fields":{"amount":"10.00","txn":"T1"}},` +
	`{"owner":"ACC-2","ts":"2026-03-01T00:00:00Z","fields":{"amount":"-10.00","txn":"T1"}}]}`

func TestRecordsHandler_Statuses(t *testing.T) {
	h := newRecordsHandlerT(t)
	require.Equal(t, http.StatusCreated, serveRecords(h, http.MethodPost, recordsPath+"/posting", balancedT1).Code)

	cases := []struct {
		name, method, target, body string
		want                       int
	}{
		{"types", http.MethodGet, recordsPath, "", http.StatusOK},
		{"list", http.MethodGet, recordsPath + "/posting?owner=ACC-1", "", http.StatusOK},
		{"hidden owner", http.MethodGet, recordsPath + "/posting?owner=HIDDEN", "", http.StatusNotFound},
		{"unknown type", http.MethodGet, recordsPath + "/nope?owner=ACC-1", "", http.StatusNotFound},
		{"bad limit", http.MethodGet, recordsPath + "/posting?owner=ACC-1&limit=x", "", http.StatusBadRequest},
		{"bad range", http.MethodGet, recordsPath + "/posting?owner=ACC-1&from=yesterday", "", http.StatusBadRequest},
		{"reused group", http.MethodPost, recordsPath + "/posting", balancedT1, http.StatusConflict},
		{"unbalanced", http.MethodPost, recordsPath + "/posting",
			`{"rows":[{"owner":"ACC-1","fields":{"amount":"1","txn":"T2"}}]}`, http.StatusUnprocessableEntity},
		{"read-only owner", http.MethodPost, recordsPath + "/posting",
			`{"rows":[{"owner":"LOCKED","fields":{"amount":"0","txn":"T3"}}]}`, http.StatusForbidden},
		{"unknown key", http.MethodPost, recordsPath + "/posting", `{"rowz":[]}`, http.StatusBadRequest},
		{"aggregate", http.MethodGet, recordsPath + "/posting/_aggregate?owner=ACC-1&field=amount", "", http.StatusOK},
		{"transaction", http.MethodGet, recordsPath + "/posting/_transaction?group=T1", "", http.StatusOK},
		{"empty group", http.MethodGet, recordsPath + "/posting/_transaction?group=", "", http.StatusBadRequest},
		{"reverse", http.MethodPost, recordsPath + "/posting/_reverse", `{"group":"T1"}`, http.StatusCreated},
		{"reverse twice", http.MethodPost, recordsPath + "/posting/_reverse", `{"group":"T1"}`, http.StatusConflict},
		{"method", http.MethodDelete, recordsPath + "/posting", "", http.StatusMethodNotAllowed},
		{"unknown route", http.MethodGet, recordsPath + "/posting/_nope", "", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveRecords(h, tc.method, tc.target, tc.body)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}
}

// After a reversal the owner's balance is zero.
func TestRecordsHandler_BalanceAfterReverse(t *testing.T) {
	h := newRecordsHandlerT(t)
	require.Equal(t, http.StatusCreated, serveRecords(h, http.MethodPost, recordsPath+"/posting", balancedT1).Code)
	require.Equal(t, http.StatusCreated,
		serveRecords(h, http.MethodPost, recordsPath+"/posting/_reverse", `{"group":"T1"}`).Code)
	rec := serveRecords(h, http.MethodGet, recordsPath+"/posting/_aggregate?owner=ACC-1&field=amount", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Points []records.Point `json:"points"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Points, 1)
	require.Equal(t, "0.00", got.Points[0].Sum)
}

func TestRecordsHandler_AbsentWithoutService(t *testing.T) {
	var h *recordsHandler
	require.Equal(t, http.StatusNotFound, serveRecords(h, http.MethodGet, recordsPath, "").Code)
}
