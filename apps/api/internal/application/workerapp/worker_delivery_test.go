package workerapp

import (
	"context"
	"testing"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/application/workflowrunapp"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/task"
	mockexecutor "fluxa-api/internal/infrastructure/executor/mock"
	"fluxa-api/internal/infrastructure/persistence/gormdb"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReleaseDeploysStgAndProdAndCompletesTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gormdb.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now()
	projectID := "project-1"
	serviceID := "service-1"
	artifactID := "artifact-1"
	if err := db.Create(&gormdb.ProjectModel{ID: projectID, Name: "Project", Key: "project", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormdb.ProjectServiceModel{ID: serviceID, ProjectID: projectID, GitlabProjectID: 1, ServiceKey: "api", DisplayName: "API", DeployTarget: "portainer", DeployConfig: gormdb.JSONMap{}, Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	deliveryRepo := gormdb.NewDeliveryRepository(db)
	if err := deliveryRepo.EnsureProjectDefaults(ctx, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := deliveryRepo.CreateArtifact(ctx, delivery.Artifact{ID: artifactID, ProjectID: projectID, ProjectServiceID: serviceID, CommitSHA: "abc", RefType: "tag", Ref: "v1", ImageRepository: "registry/api", ImageTag: "v1", ImageDigest: "sha256:abc", IdempotencyKey: "one", BuiltAt: now, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	taskRepo := gormdb.NewTaskRepository(db)
	releaseRepo := gormdb.NewReleaseRepository(db)
	workerRepo := gormdb.NewWorkerRepository(db, 1, time.Millisecond)
	runner := New("worker-test", workerRepo, zap.NewNop(), time.Millisecond)
	taskID := "task-release"
	releaseID := "release-1"
	if _, err := taskRepo.Create(ctx, task.Task{ID: taskID, ProjectID: projectID, Title: taskID, Status: task.StatusPublishing, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	rel := release.Release{ID: releaseID, ProjectID: projectID, Title: releaseID, Status: release.ReleaseQueued, CreatedAt: now, UpdatedAt: now, Tasks: []release.ReleaseTask{{ID: "rt-" + taskID, ReleaseID: releaseID, TaskID: taskID, Title: taskID, Status: string(task.StatusPublishing), CreatedAt: now}}, Services: []release.ReleaseService{{ID: "rs-" + releaseID, ReleaseID: releaseID, ProjectServiceID: serviceID, ArtifactID: artifactID, ServiceKey: "api", DisplayName: "API", DeployTarget: "portainer", DeployConfig: map[string]any{}, Ref: "sha256:abc", Status: release.ServicePending, CreatedAt: now, UpdatedAt: now}}}
	if _, err := releaseRepo.Create(ctx, rel); err != nil {
		t.Fatal(err)
	}
	if _, err := releaseRepo.CreateJob(ctx, release.ReleaseJob{ID: "job-" + releaseID, ReleaseID: releaseID, Status: release.JobPending, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	runner.SetWorkflowRunner(workflowrunapp.New(gormdb.NewWorkflowRunRepository(db), releaseRepo, deliveryRepo, mockexecutor.NewRegistry(), authorization.New(gormdb.NewProjectRepository(db))))
	for i := 0; i < 3; i++ {
		if err := runner.tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	updatedTask, _ := taskRepo.Get(ctx, taskID)
	if updatedTask.Status != task.StatusPublished {
		t.Fatalf("release did not publish task after PROD: %s", updatedTask.Status)
	}
	deployments, err := deliveryRepo.ListDeployments(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, deployment := range deployments {
		seen[deployment.Environment] = true
	}
	if !seen["stg"] || !seen["prod"] {
		t.Fatalf("release deployments = %#v, want both stg and prod", seen)
	}
}
