package metamodel_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const recordsSchema = `
version: "1"
entities:
  account:
    label: Account
    id_prefix: "ACC-"
    properties:
      name:
        type: string
      locked_until:
        type: date
records:
  posting:
    owner: account
    ledger:
      amount: amount
      group_by: txn
      commodity: currency
      closed_before: locked_until
    fields:
      amount: {type: decimal, scale: 2, required: true}
      txn: {type: string, required: true}
      currency: {type: string, required: true}
  reading:
    owner: account
    retention: 90d
    fields:
      kwh: {type: decimal}
`

func TestRecords_Parses(t *testing.T) {
	m, err := metamodel.Parse([]byte(recordsSchema))
	require.NoError(t, err)
	require.Equal(t, []string{"posting", "reading"}, m.RecordKeys())
	require.Equal(t, 90*24*time.Hour, m.Records["reading"].RetentionDuration())
	require.Equal(t, "txn", m.Records["posting"].Ledger.GroupBy)
	require.Equal(t, []string{"amount", "currency", "txn"}, m.Records["posting"].FieldNames())
}

func TestRecords_Rejects(t *testing.T) {
	base := `
version: "1"
entities:
  account:
    label: Account
    id_prefix: "ACC-"
    properties:
      name: {type: string}
records:
`
	cases := map[string]string{
		"unknown owner": "  r:\n    owner: nope\n    fields:\n      v: {type: integer}\n",
		"no fields":     "  r:\n    owner: account\n",
		"reserved":      "  r:\n    owner: account\n    fields:\n      ts: {type: integer}\n",
		"bad type":      "  r:\n    owner: account\n    fields:\n      v: {type: text}\n",
		"list":          "  r:\n    owner: account\n    fields:\n      v: {type: string, list: true}\n",
		"retention":     "  r:\n    owner: account\n    retention: soon\n    fields:\n      v: {type: integer}\n",
		"key clash":     "  account:\n    owner: account\n    fields:\n      v: {type: integer}\n",
		"ledger amount": "  r:\n    owner: account\n    ledger: {amount: v, group_by: g}\n    fields:\n" +
			"      v: {type: string, required: true}\n      g: {type: string, required: true}\n",
		"ledger group optional": "  r:\n    owner: account\n    ledger: {amount: v, group_by: g}\n    fields:\n" +
			"      v: {type: decimal, required: true}\n      g: {type: string}\n",
		"ledger retention": "  r:\n    owner: account\n    retention: 30d\n    ledger: {amount: v, group_by: g}\n" +
			"    fields:\n      v: {type: decimal, required: true}\n      g: {type: string, required: true}\n",
		"closed_before not a date": "  r:\n    owner: account\n    ledger: {amount: v, group_by: g, closed_before: name}\n" +
			"    fields:\n      v: {type: decimal, required: true}\n      g: {type: string, required: true}\n",
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := metamodel.Parse([]byte(base + block))
			require.Error(t, err)
		})
	}
}

func TestParseRetention(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"": 0, "48h": 48 * time.Hour, "90d": 90 * 24 * time.Hour, "2w": 14 * 24 * time.Hour,
	} {
		got, err := metamodel.ParseRetention(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}
	for _, in := range []string{"x", "-1d", "0d", "3m"} {
		_, err := metamodel.ParseRetention(in)
		require.Error(t, err, in)
	}
}
