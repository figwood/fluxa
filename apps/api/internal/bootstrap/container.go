package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/application/deliveryapp"
	"fluxa-api/internal/application/iamapp"
	"fluxa-api/internal/application/projectapp"
	"fluxa-api/internal/application/releaseapp"
	"fluxa-api/internal/application/taskapp"
	"fluxa-api/internal/application/workbenchapp"
	"fluxa-api/internal/application/workerapp"
	"fluxa-api/internal/application/workflowapp"
	"fluxa-api/internal/application/workflowrunapp"
	"fluxa-api/internal/config"
	mockexecutor "fluxa-api/internal/infrastructure/executor/mock"
	"fluxa-api/internal/infrastructure/gitlabclient"
	"fluxa-api/internal/infrastructure/persistence/gormdb"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Container struct {
	Config       config.Config
	Logger       *zap.Logger
	Projects     *projectapp.Service
	Catalog      *projectapp.CatalogService
	Delivery     *deliveryapp.Service
	Tasks        *taskapp.Service
	Releases     *releaseapp.Service
	Workflows    *workflowapp.Service
	WorkflowRuns *workflowrunapp.Service
	IAM          *iamapp.Service
	Worker       *workerapp.Worker
	Workers      *workerapp.ControlService
	Workbench    *workbenchapp.Service
	DB           *gorm.DB
}

func New(cfg config.Config, log *zap.Logger) (*Container, error) {
	if strings.EqualFold(cfg.AppEnv, "production") && len(strings.TrimSpace(cfg.JWTSecret)) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must contain at least 32 characters in production")
	}
	if strings.EqualFold(cfg.AppEnv, "production") && cfg.AutoMigrate {
		return nil, fmt.Errorf("AUTO_MIGRATE must be false in production; apply versioned migrations first")
	}
	db, err := gormdb.Open(cfg)
	if err != nil {
		return nil, err
	}
	projectRepo := gormdb.NewProjectRepository(db)
	catalogRepo := gormdb.NewServiceCatalogRepository(db)
	if err := catalogRepo.BackfillProjectGitlabRepositories(context.Background()); err != nil {
		return nil, err
	}
	taskRepo := gormdb.NewTaskRepository(db)
	if err := taskRepo.BackfillCreators(context.Background()); err != nil {
		return nil, err
	}
	releaseRepo := gormdb.NewReleaseRepository(db)
	workerRepo := gormdb.NewWorkerRepository(db, cfg.WorkerReplicas, cfg.WorkerPollInterval)
	workflowRepo := gormdb.NewWorkflowRepository(db)
	workflowRunRepo := gormdb.NewWorkflowRunRepository(db)
	deliveryRepo := gormdb.NewDeliveryRepository(db)
	iamRepo := gormdb.NewIAMRepository(db)
	iamRepo.SetProduction(strings.EqualFold(cfg.AppEnv, "production"))
	iamSvc := iamapp.New(iamRepo, cfg.JWTSecret, cfg.AccessTokenTTL)
	if err := iamSvc.EnsureDefaults(context.Background()); err != nil {
		return nil, err
	}
	executors := mockexecutor.NewRegistry()
	accessSvc := authorization.New(projectRepo)
	workflowSvc := workflowapp.New(workflowRepo, accessSvc)
	if err := workflowSvc.EnsureDefaults(context.Background()); err != nil {
		return nil, err
	}
	gitlab := gitlabclient.New(cfg.GitlabURL, cfg.GitlabToken, cfg.GitlabTimeout)
	projects := projectapp.New(projectRepo, accessSvc)
	tasks := taskapp.New(taskRepo, projectRepo, workflowRepo, accessSvc)
	releases := releaseapp.New(releaseRepo, taskRepo, catalogRepo, workflowRepo, deliveryRepo, accessSvc)
	workflowRuns := workflowrunapp.New(workflowRunRepo, releaseRepo, deliveryRepo, executors, accessSvc)
	releases.SetWorkflowRunner(workflowRuns)
	worker := workerapp.New(cfg.WorkerID, workerRepo, log, cfg.WorkerPollInterval)
	worker.SetWorkflowRunner(workflowRuns)
	return &Container{
		Config: cfg, Logger: log, DB: db,
		Projects:     projects,
		Catalog:      projectapp.NewCatalog(catalogRepo, accessSvc, gitlab),
		Delivery:     deliveryapp.New(deliveryRepo, catalogRepo, accessSvc, executors),
		Tasks:        tasks,
		Releases:     releases,
		Workbench:    workbenchapp.New(projects, tasks, releases),
		Workflows:    workflowSvc,
		WorkflowRuns: workflowRuns,
		IAM:          iamSvc,
		Worker:       worker,
		Workers:      workerapp.NewControl(workerRepo),
	}, nil
}
