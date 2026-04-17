package graph

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// TestPropertyIndexConsistency uses property-based testing to verify that
// the property index remains consistent across all graph mutations
func TestPropertyIndexConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property 1: Adding and removing the same entity should result in empty index
	properties.Property("AddNode then RemoveNode restores empty index", prop.ForAll(
		func(entityID string, entityType string, propName string, propValue string) bool {
			g := New()

			entity := &model.Entity{
				ID:   entityID,
				Type: entityType,
				Properties: map[string]interface{}{
					propName: propValue,
				},
			}

			// Add the entity
			g.AddNode(entity)

			// Check index is populated
			if g.propertyIndex[propName] == nil || g.propertyIndex[propName][propValue] != 1 {
				return false
			}

			// Remove the entity
			g.RemoveNode(entityID)

			// Check index is cleaned up
			return len(g.propertyIndex[propName]) == 0 && len(g.propertyIndex) == 0
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property 2: UpdateNode should correctly update the index
	properties.Property("UpdateNode maintains index consistency", prop.ForAll(
		func(entityID string, entityType string, propName string, oldValue string, newValue string) bool {
			g := New()

			oldEntity := &model.Entity{
				ID:   entityID,
				Type: entityType,
				Properties: map[string]interface{}{
					propName: oldValue,
				},
			}

			// Add original entity
			g.AddNode(oldEntity)

			// Verify old value in index
			if g.propertyIndex[propName][oldValue] != 1 {
				return false
			}

			newEntity := &model.Entity{
				ID:   entityID,
				Type: entityType,
				Properties: map[string]interface{}{
					propName: newValue,
				},
			}

			// Update entity
			g.UpdateNode(newEntity)

			// Verify old value removed from index
			if oldValue != newValue && g.propertyIndex[propName][oldValue] != 0 {
				return false
			}

			// Verify new value in index
			return g.propertyIndex[propName][newValue] == 1
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property 3: Multiple entities with same property value should increment count
	properties.Property("Multiple entities increment property value count", prop.ForAll(
		func(propName string, propValue string, count uint8) bool {
			if count == 0 {
				return true // Skip zero count
			}

			g := New()

			// Add multiple entities with same property value
			for i := uint8(0); i < count; i++ {
				entity := &model.Entity{
					ID:   fmt.Sprintf("entity-%d", i),
					Type: "testType",
					Properties: map[string]interface{}{
						propName: propValue,
					},
				}
				g.AddNode(entity)
			}

			// Verify count in index
			return g.propertyIndex[propName][propValue] == int(count)
		},
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.UInt8(),
	))

	// Property 4: Removing one of many entities with same value decrements count
	properties.Property("Removing entity decrements shared property value count", prop.ForAll(
		func(propName string, propValue string, totalCount uint8, removeIdx uint8) bool {
			if totalCount == 0 || totalCount > 100 {
				return true // Skip invalid counts
			}
			if removeIdx >= totalCount {
				return true // Skip invalid index
			}

			g := New()

			// Add multiple entities with same property value
			for i := uint8(0); i < totalCount; i++ {
				entity := &model.Entity{
					ID:   fmt.Sprintf("entity-%d", i),
					Type: "testType",
					Properties: map[string]interface{}{
						propName: propValue,
					},
				}
				g.AddNode(entity)
			}

			// Remove one entity
			g.RemoveNode(fmt.Sprintf("entity-%d", removeIdx))

			expectedCount := int(totalCount) - 1

			// If count should be 0, map should be cleaned up
			if expectedCount == 0 {
				return len(g.propertyIndex[propName]) == 0
			}

			// Otherwise, verify decremented count
			return g.propertyIndex[propName][propValue] == expectedCount
		},
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.UInt8Range(1, 100),
		gen.UInt8(),
	))

	// Property 5: GetPropertyValues returns values sorted by frequency
	properties.Property("GetPropertyValues returns frequency-sorted values", prop.ForAll(
		func(propName string, val1Seed string, val2Seed string, val3Seed string) bool {
			// Ensure unique values
			val1 := val1Seed + "-1"
			val2 := val2Seed + "-2"
			val3 := val3Seed + "-3"

			g := New()

			// Add val1 once, val2 twice, val3 three times
			values := []struct {
				val   string
				count int
			}{
				{val1, 1},
				{val2, 2},
				{val3, 3},
			}

			for _, v := range values {
				for i := 0; i < v.count; i++ {
					entity := &model.Entity{
						ID:   fmt.Sprintf("entity-%s-%d", v.val, i),
						Type: "testType",
						Properties: map[string]interface{}{
							propName: v.val,
						},
					}
					g.AddNode(entity)
				}
			}

			// Get values
			result := g.GetPropertyValues(propName, 10)

			if len(result) != 3 {
				return false
			}

			// val3 should be first (highest frequency)
			return result[0] == val3 && result[1] == val2 && result[2] == val1
		},
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property 6: Property index survives multiple add/update/remove cycles
	properties.Property("Index consistency across mixed operations", prop.ForAll(
		func(numOps uint8, entityID string, propName string, value1 string, value2 string) bool {
			if numOps == 0 || numOps > 20 {
				return true
			}

			g := New()
			exists := false

			entity1 := &model.Entity{
				ID:   entityID,
				Type: "testType",
				Properties: map[string]interface{}{
					propName: value1,
				},
			}

			entity2 := &model.Entity{
				ID:   entityID,
				Type: "testType",
				Properties: map[string]interface{}{
					propName: value2,
				},
			}

			// Execute operations based on index
			for i := uint8(0); i < numOps; i++ {
				switch i % 3 {
				case 0: // Add
					if !exists {
						g.AddNode(entity1)
						exists = true
					}
				case 1: // Update
					if exists {
						g.UpdateNode(entity2)
					}
				default: // Remove
					if exists {
						g.RemoveNode(entityID)
						exists = false
					}
				}
			}

			// Verify index consistency
			if !exists {
				// If entity was removed, index should be empty or not contain our values
				if g.propertyIndex[propName] != nil {
					if g.propertyIndex[propName][value1] > 0 || g.propertyIndex[propName][value2] > 0 {
						return false
					}
				}
			} else {
				// If entity exists, exactly one value should be in index
				count1 := 0
				count2 := 0
				if g.propertyIndex[propName] != nil {
					count1 = g.propertyIndex[propName][value1]
					count2 = g.propertyIndex[propName][value2]
				}

				// Exactly one should be 1, the other should be 0
				return (count1 == 1 && count2 == 0) || (count1 == 0 && count2 == 1)
			}

			return true
		},
		gen.UInt8(),
		gen.Identifier(),
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property 7: Different property types are correctly indexed
	properties.Property("Different value types are correctly indexed", prop.ForAll(
		func(entityID string, propName string, intVal int, floatVal float64, boolVal bool, strVal string) bool {
			g := New()

			// Test with different types
			testCases := []interface{}{
				intVal,
				floatVal,
				boolVal,
				strVal,
			}

			for i, val := range testCases {
				entity := &model.Entity{
					ID:   fmt.Sprintf("%s-%d", entityID, i),
					Type: "testType",
					Properties: map[string]interface{}{
						propName: val,
					},
				}

				g.AddNode(entity)

				// Verify value is indexed (converted to string)
				strValue := g.valueToString(val)
				if strValue == "" {
					continue
				}

				if g.propertyIndex[propName][strValue] != 1 {
					return false
				}

				// Clean up for next iteration
				g.RemoveNode(entity.ID)
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Int(),
		gen.Float64(),
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	properties.TestingRun(t)
}

// Helper types and generators

type testEntity struct {
	ID        string
	Type      string
	PropName  string
	PropValue string
}

func genTestEntities() gopter.Gen {
	return func(params *gopter.GenParameters) *gopter.GenResult {
		// Generate a count between 1 and 10
		countGen := gen.UInt8Range(1, 10)(params)
		n := int(countGen.Result.(uint8))

		result := make([]testEntity, n)
		for i := 0; i < n; i++ {
			result[i] = testEntity{
				ID:        fmt.Sprintf("entity-%d", i),
				Type:      "testType",
				PropName:  "status",
				PropValue: fmt.Sprintf("value-%d", i%3), // Ensure some duplicates
			}
		}
		return gopter.NewGenResult(result, gopter.NoShrinker)
	}
}
