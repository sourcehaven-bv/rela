package pgstore

import (
	"strings"
	"testing"
)

// TestListenerConnConfig_StripsPoolKeys pins the cause of BUG-JQO2PH without a
// database: a pgxpool-only key left in RuntimeParams is sent to the server,
// which refuses the listener's connection.
func TestListenerConnConfig_StripsPoolKeys(t *testing.T) {
	for _, dsn := range []string{
		"postgres://u@db.example:5432/rela?sslmode=disable&pool_max_conns=4&pool_min_conns=1&application_name=x",
		"host=db.example port=5432 user=u dbname=rela sslmode=disable pool_max_conns=4 pool_min_conns=1 application_name=x",
	} {
		cfg, err := listenerConnConfig(dsn)
		if err != nil {
			t.Fatalf("%s: %v", dsn, err)
		}
		for k := range cfg.RuntimeParams {
			if strings.HasPrefix(k, "pool_") {
				t.Errorf("%s: pool key %q would reach the server", dsn, k)
			}
		}
		if cfg.RuntimeParams["application_name"] != "x" {
			t.Errorf("%s: server parameter application_name lost: %v", dsn, cfg.RuntimeParams)
		}
	}
}
