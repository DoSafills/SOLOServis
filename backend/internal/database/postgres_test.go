package database

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConfigurePoolBoundsDatabaseWork(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://user:password@localhost:5432/soloservis")
	if err != nil {
		t.Fatal(err)
	}

	configurePool(config)

	if config.MaxConns != 10 || config.MinConns != 2 {
		t.Fatalf("pool bounds = %d/%d, want 2/10", config.MinConns, config.MaxConns)
	}
	if config.ConnConfig.ConnectTimeout != 5*time.Second {
		t.Fatalf("connection timeout = %s, want 5s", config.ConnConfig.ConnectTimeout)
	}
	if got := config.ConnConfig.RuntimeParams["statement_timeout"]; got != "30s" {
		t.Fatalf("statement timeout = %q, want 30s", got)
	}
}
