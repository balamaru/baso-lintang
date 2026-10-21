//go:build integration

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"baso-lintang/internal/domain"
	"baso-lintang/internal/store"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Database always creates a disposable container; it never uses an existing database.
func Database(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:18.6-alpine",
			Env:          map[string]string{"POSTGRES_DB": "lintang_test", "POSTGRES_USER": "test", "POSTGRES_PASSWORD": "test-only-password"},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90 * time.Second),
		}, Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Errorf("terminate test container: %v", err)
		}
	})
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	dsn := fmt.Sprintf("postgres://test:test-only-password@%s:%s/lintang_test?sslmode=disable", host, port.Port())
	// Verify both migration directions only on this newly created test database.
	for _, direction := range []string{"up", "down", "up"} {
		if err := store.Migrate(ctx, dsn, direction); err != nil {
			t.Fatalf("migrate %s: %v", direction, err)
		}
	}
	s, err := store.Open(ctx, dsn, domain.Normalizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Pool.Close)
	return s
}
