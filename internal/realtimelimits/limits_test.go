package realtimelimits

import (
	"testing"
	"time"
)

func TestParseRate(t *testing.T) {
	tests := []struct {
		in      string
		want    Rate
		wantErr bool
	}{
		{"30/m", Rate{30, time.Minute}, false},
		{"2/s", Rate{2, time.Second}, false},
		{" 100/h ", Rate{100, time.Hour}, false},
		{"30", Rate{}, true},
		{"0/m", Rate{}, true},
		{"-1/m", Rate{}, true},
		{"x/m", Rate{}, true},
		{"3/d", Rate{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseRate(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseRate(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("ParseRate(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRate_AtMostAndString(t *testing.T) {
	if !(Rate{30, time.Hour}).AtMost(DefaultLogRate) {
		t.Error("30/h should be at most 30/m")
	}
	if (Rate{1000, time.Hour}).AtMost(DefaultLogRate) {
		t.Error("1000/h has a lower average than 30/m but a larger burst; it should not be at most 30/m")
	}
	if (Rate{1, time.Second}).AtMost(DefaultLogRate) {
		t.Error("1/s should exceed 30/m")
	}
	if (Rate{}).PerSecond() != 0 {
		t.Error("a zero rate should be zero per second")
	}
	for _, r := range []Rate{DefaultLogRate, DefaultEphemeralRate, {5, time.Hour}} {
		back, err := ParseRate(r.String())
		if err != nil || back != r {
			t.Errorf("round trip %v: got %v, %v", r, back, err)
		}
	}
}

// TestTypingLimits pins that the typing numbers stay inside the signal caps.
func TestTypingLimits(t *testing.T) {
	if TypingTTL > MaxEphemeralTTL {
		t.Errorf("TypingTTL %v exceeds MaxEphemeralTTL %v", TypingTTL, MaxEphemeralTTL)
	}
	if !TypingRate.AtMost(DefaultEphemeralRate) {
		t.Errorf("TypingRate %v exceeds DefaultEphemeralRate %v", TypingRate, DefaultEphemeralRate)
	}
}

func TestParseSize(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"1000", 1000},
		{"512KiB", 512 << 10},
		{" 5 MiB ", 5 << 20},
		{"0", 0},
		{"-1", 0},
		{"5MB", 0},
		{"1.5MiB", 0},
		{"", 0},
		{"2000000MiB", 0},
	} {
		got, err := ParseSize(tc.in)
		if tc.want == 0 {
			if err == nil {
				t.Errorf("ParseSize(%q) = %d, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseSize(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}
