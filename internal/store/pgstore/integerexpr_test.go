package pgstore_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/propmatch"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestIntegerExprAgreesWithGo runs the SQL reading of a stored integer
// (store.OrderInteger) on JSON texts another tool may have written, and
// compares it with what a read decodes and propmatch.Integer accepts
// (TKT-8HY05WB). The storetest suites only see values Go wrote.
func TestIntegerExprAgreesWithGo(t *testing.T) {
	pool := newScopedPool(t)
	expr := pgstore.IntegerExprForTest("(v.j->>'p')", "(v.j->'p')")
	for _, text := range []string{
		`5`, `-4`, `0`, `-0`, `0.0`, `0e5`, `5.0`, `5e0`, `50e-1`, `2.5`, `-2.5`, `0.5`, `1e-400`, `2e-324`, `3e-324`,
		`4.99999999999999999999`, `9223372036854775807`, `-9223372036854775808`,
		`9223372036854775808`, `9223372036854775807.0`, `9.223372036854775807e18`, `1e19`, `-1e19`,
		`1e400`, `123456789012345678`, `1.5e15`,
		`"7"`, `"+3"`, `"-3"`, `"007"`, `" 7"`, `"7 "`, `"x"`, `""`, `"2.5"`, `"9223372036854775807"`,
		`"9223372036854775808"`, `"-9223372036854775809"`, `"-000"`, `"+0"`, `"00000000000000000000000000042"`,
		`"-00000000000009223372036854775808"`, `"10000000000000000000"`,
		`"` + strings.Repeat("0", 200000) + `7"`, `"` + strings.Repeat("9", 200000) + `"`,
		`"` + strings.Repeat("0", 200000) + `"`, `"-` + strings.Repeat("0", 200000) + `9223372036854775809"`,
		`true`, `null`, `[]`, `[1]`, `{"a": 1}`,
	} {
		name := text
		if len(name) > 40 {
			name = name[:20] + "...(" + strconv.Itoa(len(text)) + " bytes)"
		}
		t.Run(name, func(t *testing.T) {
			// jsonb keeps a number as numeric, not as its text, so Go
			// decodes what PostgreSQL gives back.
			var stored string
			var got *string
			err := pool.QueryRow(context.Background(),
				"SELECT v.j::text, ("+expr+")::text FROM (SELECT $1::jsonb AS j) v",
				`{"p": `+text+`}`).Scan(&stored, &got)
			require.NoError(t, err)
			props, err := pgstore.UnmarshalPropsForTest([]byte(stored))
			require.NoError(t, err)
			want, wantOK := propmatch.Integer(props["p"])
			if !wantOK {
				require.Nil(t, got, "Go reads no integer from %s", text)
				return
			}
			require.NotNil(t, got, "Go reads %d from %s", want, text)
			require.Equal(t, strconv.FormatInt(want, 10), *got)
		})
	}
}
