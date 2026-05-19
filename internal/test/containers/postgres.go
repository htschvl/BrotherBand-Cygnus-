// Package containers wraps Testcontainers helpers used by the
// repository-layer integration tests. Containers are session-scoped
// (one per `go test` invocation) via WithReuse to amortise the
// cold-start cost across hundreds of tests.
package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Postgres bundles the running container and a ready-to-use DSN.
type Postgres struct {
	Container testcontainers.Container
	DSN       string
}

// StartPostgres boots a postgres:16-alpine container and waits for
// it to be ready. Pass the returned Container.Terminate to t.Cleanup
// in TestMain.
func StartPostgres(ctx context.Context) (*Postgres, error) {
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("brotherband"),
		postgres.WithUsername("brotherband"),
		postgres.WithPassword("brotherband"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("containers: start postgres: %w", err)
	}
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("containers: get postgres dsn: %w", err)
	}
	return &Postgres{Container: pg, DSN: dsn}, nil
}
