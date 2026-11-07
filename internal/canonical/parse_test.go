package canonical_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestParseDigest(t *testing.T) {
	d := canonical.SumEntity(*entity.New("T-1", "ticket"))
	got, err := canonical.ParseDigest(d.String())
	if err != nil || got != d {
		t.Fatalf("ParseDigest(String()) = %v, %v; want %v", got, err, d)
	}
	for name, s := range map[string]string{
		"empty":      "",
		"short":      d.String()[:63],
		"long":       d.String() + "0",
		"upper case": strings.ToUpper(d.String()),
		"not hex":    "g" + d.String()[1:],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := canonical.ParseDigest(s); err == nil {
				t.Errorf("ParseDigest(%q) accepted", s)
			}
		})
	}
}
