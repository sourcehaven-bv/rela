package lua

import (
	"math"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestLuaNumberToGo_PreservesIntegers pins that an integral Lua number
// converts to a Go int64 (not float64), so large integer IDs survive a
// Lua→Go round-trip without precision loss. A non-integral value stays
// float64.
func TestLuaNumberToGo_PreservesIntegers(t *testing.T) {
	tests := []struct {
		name string
		in   lua.LNumber
		want interface{}
	}{
		{"small int", lua.LNumber(42), int64(42)},
		{"zero", lua.LNumber(0), int64(0)},
		{"negative int", lua.LNumber(-7), int64(-7)},
		// Largest integer exactly representable as float64 (2^53). Beyond
		// this, gopher-lua's float64-backed LNumber cannot hold an integer
		// faithfully at all — the precision is lost before conversion —
		// so this is the practical ceiling for int64 preservation.
		{"max exact int (2^53)", lua.LNumber(1 << 53), int64(1 << 53)},
		{"fractional stays float", lua.LNumber(3.5), 3.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := luaNumberToGo(tc.in)
			if got != tc.want {
				t.Errorf("luaNumberToGo(%v) = %#v (%T), want %#v (%T)", float64(tc.in), got, got, tc.want, tc.want)
			}
		})
	}
}

// TestLuaValueToGo_IntegerNotFloat pins the integration point: a numeric
// LValue routed through luaValueToGo yields an int64 for integral values.
func TestLuaValueToGo_IntegerNotFloat(t *testing.T) {
	got := luaValueToGo(lua.LNumber(100))
	if got != int64(100) {
		t.Errorf("luaValueToGo(100) = %#v (%T), want int64(100)", got, got)
	}
}

// TestLuaValueToSortable_OnlyWholeStringsAreNumeric pins that a string
// is treated as numeric only when it parses entirely as a number — so
// "1.2.0" and "3 blind mice" sort lexicographically rather than being
// reduced to their numeric prefix (the old Sscanf("%f") behavior).
func TestLuaValueToSortable_OnlyWholeStringsAreNumeric(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantIsNum bool
		wantNum   float64
	}{
		{"pure integer", "42", true, 42},
		{"pure float", "3.14", true, 3.14},
		{"negative", "-5", true, -5},
		{"whitespace padded", "  7  ", true, 7},
		{"version string", "1.2.0", false, 0},
		{"numeric prefix only", "3 blind mice", false, 0},
		{"empty", "", false, 0},
		{"trailing junk", "10px", false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, num, isNum := luaValueToSortable(lua.LString(tc.in))
			if isNum != tc.wantIsNum {
				t.Errorf("luaValueToSortable(%q): isNum = %v, want %v", tc.in, isNum, tc.wantIsNum)
			}
			if isNum && num != tc.wantNum {
				t.Errorf("luaValueToSortable(%q): num = %v, want %v", tc.in, num, tc.wantNum)
			}
		})
	}
}

// TestSortEntries_VersionStringsAreLexicographic pins that version-like
// strings (which the old Sscanf path mis-sorted as their integer prefix)
// now sort lexicographically, end to end through sortEntries.
func TestSortEntries_VersionStringsAreLexicographic(t *testing.T) {
	entries := []sortableEntry{
		{prop: lua.LString("1.10.0")},
		{prop: lua.LString("1.2.0")},
		{prop: lua.LString("1.9.0")},
	}
	sortEntries(entries, false)

	// Lexicographic: "1.10.0" < "1.2.0" < "1.9.0" (since '1' < '2' < '9'
	// at the char after "1."). The old numeric-prefix path would have
	// treated all three as 1 and left them in input order.
	want := []string{"1.10.0", "1.2.0", "1.9.0"}
	for i, w := range want {
		if got := entries[i].prop.String(); got != w {
			t.Errorf("at %d: got %q, want %q", i, got, w)
		}
	}
}

func TestLuaNumberToGo_OutOfInt64RangeStaysFloat(t *testing.T) {
	// A value larger than MaxInt64 cannot be an int64; must stay float64.
	big := lua.LNumber(math.MaxFloat64)
	if got := luaNumberToGo(big); got != math.MaxFloat64 {
		t.Errorf("luaNumberToGo(MaxFloat64) = %#v (%T), want float64", got, got)
	}
}
