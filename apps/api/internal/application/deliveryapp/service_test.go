package deliveryapp

import (
	"context"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/servicecatalog"
	mockexecutor "fluxa-api/internal/infrastructure/executor/mock"
	"fluxa-api/internal/infrastructure/persistence/gormdb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestFailedExecutorResultIsRecordedAsFailure(t *testing.T) {
	db, e := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if e = gormdb.AutoMigrate(db); e != nil {
		t.Fatal(e)
	}
	repo := gormdb.NewDeliveryRepository(db)
	svc := New(repo, nil, nil, mockexecutor.NewRegistry())
	dep, e := svc.deploy(context.Background(), servicecatalog.ProjectService{ID: "s", ProjectID: "p", DeployTarget: "portainer", DeployConfig: map[string]any{"mock_result": "failed"}}, delivery.Artifact{ID: "a", ImageDigest: "sha256:a"}, "dev", "ci_auto", "")
	if e == nil || dep.Status != "failed" {
		t.Fatalf("failed executor marked %s: %v", dep.Status, e)
	}
	rows, e := repo.ListDeployments(context.Background(), "p")
	if e != nil || len(rows) != 1 || rows[0].Status != "failed" {
		t.Fatalf("%+v %v", rows, e)
	}
}
