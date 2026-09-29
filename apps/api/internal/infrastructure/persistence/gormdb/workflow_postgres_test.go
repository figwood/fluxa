package gormdb

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Run only against an explicitly supplied test database. Each run owns a schema;
// application configuration and existing schemas are never used or modified.
func TestPostgresConcurrentStartAndClaim(t *testing.T) {
	dsn := os.Getenv("FLUXA_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FLUXA_TEST_POSTGRES_DSN not configured")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	adminSQL := stdlib.OpenDB(*cfg)
	defer adminSQL.Close()
	schema := shared.NewID("fluxa_test")
	if _, err = adminSQL.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Exec("DROP SCHEMA " + schema + " CASCADE")
	cfg.RuntimeParams["search_path"] = schema
	conn := stdlib.OpenDB(*cfg)
	defer conn.Close()
	conn.SetMaxOpenConns(8)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err = AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../../migrations/0004_workflow_deployment_operations.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now()
	if err = db.Create(&ReleaseModel{ID: "r", ProjectID: "p", Title: "R", Status: "approved", Details: JSONMap{}, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewWorkflowRunRepository(db)
	def := workflowrun.DefaultReleaseDefinition()
	var wg sync.WaitGroup
	results := make(chan workflowrun.Execution, 8)
	failures := make(chan error, 8)
	begin := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-begin
			x, e := repo.Create(ctx, workflowrun.Execution{ID: shared.NewID("wfexec"), ReleaseID: "r", ProjectID: "p", DefinitionName: def.Name, DefinitionVersion: def.Version, DefinitionSnapshot: def, PublicStatus: "publishing", Phase: workflowrun.PhaseRunning, CurrentStepID: def.Start, Context: map[string]any{"deployment_tracking_v1": true}, LockVersion: 1, CreatedAt: now, UpdatedAt: now})
			if e != nil {
				failures <- e
			} else {
				results <- x
			}
		}()
	}
	close(begin)
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	id := ""
	for x := range results {
		if id != "" && id != x.ID {
			t.Fatal("duplicate executions")
		}
		id = x.ID
	}
	claims := make(chan workflowrun.Execution, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x, ok, e := repo.ClaimNext(ctx)
			if e != nil {
				errs <- e
			} else if ok {
				claims <- x
			}
		}()
	}
	wg.Wait()
	close(claims)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if len(claims) != 1 {
		t.Fatalf("claimed %d times", len(claims))
	}
	old := <-claims
	if err = repo.RenewLease(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&WorkflowExecutionModel{}).Where("id = ?", id).Update("wake_up_at", now.Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok, e := repo.ClaimNext(ctx); e != nil || !ok {
		t.Fatalf("reclaim %v %v", ok, e)
	}
	if err = repo.RenewLease(ctx, old); err == nil {
		t.Fatal("stale lease renewed")
	}
}
