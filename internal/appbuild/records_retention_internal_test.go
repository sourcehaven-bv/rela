package appbuild

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/records"
)

type openOwners struct{}

func (openOwners) Readable(_ context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (openOwners) Writable(_ context.Context, id string) (records.Owner, error) {
	return records.Owner{ID: id, Type: "meter"}, nil
}

// TestRecordRetention_DropsExpiredRows runs the retention job through a real
// queue: a row past the retention is dropped, a recent one stays.
func TestRecordRetention_DropsExpiredRows(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  meter:
    label: Meter
    id_prefix: "MTR-"
    properties:
      title: {type: string}
records:
  reading:
    owner: meter
    retention: 24h
    fields:
      kwh: {type: integer}
`))
	require.NoError(t, err)
	svc, err := records.NewService(records.Deps{Store: memoryRecords(), Owners: openOwners{}, Meta: meta})
	require.NoError(t, err)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err = svc.Append(ctx, "reading", []records.Input{
		{Owner: "MTR-1", TS: now.Add(-48 * time.Hour), Fields: map[string]any{"kwh": 1}},
		{Owner: "MTR-1", TS: now, Fields: map[string]any{"kwh": 2}},
	})
	require.NoError(t, err)

	q, err := jobs.NewMemoryQueue(ctx, slog.Default())
	require.NoError(t, err)
	require.NoError(t, q.Start(ctx))
	t.Cleanup(func() { closeJobQueue(q) })
	stop := startRecordRetention(svc, q, 20*time.Millisecond)
	t.Cleanup(stop)

	require.Eventually(t, func() bool {
		page, err := svc.List(ctx, "reading", "MTR-1", records.ListQuery{})
		return err == nil && len(page.Records) == 1 && page.Records[0].Fields["kwh"] == int64(2)
	}, 5*time.Second, 20*time.Millisecond)
}

// TestRecordRetention_IdleWithoutRetention starts nothing when no record
// type declares a retention.
func TestRecordRetention_IdleWithoutRetention(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  meter:
    label: Meter
    id_prefix: "MTR-"
    properties:
      title: {type: string}
records:
  reading:
    owner: meter
    fields:
      kwh: {type: integer}
`))
	require.NoError(t, err)
	svc, err := records.NewService(records.Deps{Store: memoryRecords(), Owners: openOwners{}, Meta: meta})
	require.NoError(t, err)
	require.False(t, hasRetention(svc))
	startRecordRetention(svc, nil, time.Millisecond)()
}
