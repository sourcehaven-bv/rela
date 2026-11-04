package storetest

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// RunDecimalTextTests checks that a decimal property value, which rela
// stores as text in decimal notation (metamodel.PropertyTypeDecimal), comes
// back exactly as written. A backend that let "12.50" turn into the number
// 12.5 would lose the trailing zero and change the value's type.
func RunDecimalTextTests(t *testing.T, f Factory) {
	values := map[string]any{
		"price":    "12.50",
		"small":    "-0.125",
		"whole":    "100",
		"long":     "12345678901234567890.123456789012345678",
		"leading":  "007.10",
		"amounts":  []any{"1.0", "2.50"},
		"positive": "+3.0",
	}
	s := f(t)
	e := entity.New("ITEM-DEC", "item")
	maps.Copy(e.Properties, values)
	require.NoError(t, s.CreateEntity(ctx(), e))

	got, err := s.GetEntity(ctx(), e.Ref())
	require.NoError(t, err)
	assertDecimalText(t, values, got)

	next := got.Clone()
	next.Properties["price"] = "12.500"
	require.NoError(t, s.UpdateEntity(ctx(), next))
	got, err = s.GetEntity(ctx(), e.Ref())
	require.NoError(t, err)
	values["price"] = "12.500"
	assertDecimalText(t, values, got)
}

func assertDecimalText(t *testing.T, want map[string]any, got *entity.Entity) {
	t.Helper()
	for k, v := range want {
		gv := got.Properties[k]
		if list, ok := v.([]any); ok {
			var texts []any
			switch g := gv.(type) {
			case []any:
				texts = g
			case []string:
				for _, s := range g {
					texts = append(texts, s)
				}
			}
			assert.Equal(t, list, texts, "property %s", k)
			continue
		}
		assert.Equal(t, v, gv, "property %s", k)
	}
}
