package workflowrunapp

import (
	"context"
	"testing"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/application/releaseapp"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/executor"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/infrastructure/persistence/gormdb"
	"fluxa-api/internal/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testRegistry struct{ exec executor.Executor }

func (r testRegistry) Get(string) (executor.Executor, bool) { return r.exec, true }

type deployFunc func(context.Context, executor.DeployJob) (*executor.TriggerResult, error)

func (f deployFunc) Trigger(c context.Context, j executor.DeployJob) (*executor.TriggerResult, error) {
	return f(c, j)
}

type fixture struct {
	db       *gorm.DB
	ctx      context.Context
	svc      *Service
	repo     *gormdb.WorkflowRunRepository
	releases *gormdb.ReleaseRepository
	delivery *gormdb.DeliveryRepository
	tasks    *gormdb.TaskRepository
}

func newFixture(t *testing.T, exec executor.Executor) fixture {
	t.Helper()
	db, e := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	if e = gormdb.AutoMigrate(db); e != nil {
		t.Fatal(e)
	}
	ctx := security.WithPrincipal(context.Background(), security.Principal{UserID: 1, UserEmail: "owner@example.com", GlobalRoles: []string{"admin"}})
	db.Create(&gormdb.ProjectModel{ID: "p", Name: "P", Key: "p"})
	rr := gormdb.NewReleaseRepository(db)
	dr := gormdb.NewDeliveryRepository(db)
	tr := gormdb.NewTaskRepository(db)
	wr := gormdb.NewWorkflowRunRepository(db)
	if e = dr.EnsureProjectDefaults(ctx, "p"); e != nil {
		t.Fatal(e)
	}
	if _, e = tr.Create(ctx, task.Task{ID: "t", ProjectID: "p", Title: "T", Status: task.StatusPublishing}); e != nil {
		t.Fatal(e)
	}
	if _, e = dr.CreateArtifact(ctx, delivery.Artifact{ID: "a", ProjectID: "p", ProjectServiceID: "s", ImageDigest: "sha256:test", IdempotencyKey: "a"}); e != nil {
		t.Fatal(e)
	}
	if _, e = rr.Create(ctx, release.Release{ID: "r", ProjectID: "p", Title: "R", Status: release.ReleaseApproved, Tasks: []release.ReleaseTask{{ID: "rt", ReleaseID: "r", TaskID: "t"}}, Services: []release.ReleaseService{{ID: "rs", ReleaseID: "r", ProjectServiceID: "s", ArtifactID: "a", DeployTarget: "portainer", DeployConfig: map[string]any{}, Status: release.ServicePending}}}); e != nil {
		t.Fatal(e)
	}
	if exec == nil {
		exec = deployFunc(func(context.Context, executor.DeployJob) (*executor.TriggerResult, error) {
			return &executor.TriggerResult{Status: "success", ExternalID: "external"}, nil
		})
	}
	svc := New(wr, rr, dr, testRegistry{exec}, authorization.New(gormdb.NewProjectRepository(db)))
	return fixture{db, ctx, svc, wr, rr, dr, tr}
}
func start(t *testing.T, f fixture, def *workflowrun.Definition) workflowrun.Execution {
	t.Helper()
	x, e := f.svc.Start(f.ctx, "r", workflowrun.StartInput{Definition: def})
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func tick(t *testing.T, f fixture) {
	t.Helper()
	if e := f.svc.Tick(f.ctx); e != nil {
		t.Fatal(e)
	}
}
func TestCompletionRecordsDeploymentsAndPublishesTasks(t *testing.T) {
	f := newFixture(t, nil)
	start(t, f, nil)
	tick(t, f)
	tick(t, f)
	tick(t, f)
	r, _ := f.releases.Get(f.ctx, "r")
	task, _ := f.tasks.Get(f.ctx, "t")
	deps, _ := f.delivery.ListDeployments(f.ctx, "p")
	if r.Status != release.ReleaseSuccess || task.Status != "published" || len(deps) != 2 {
		t.Fatalf("release=%s task=%s deployments=%d", r.Status, task.Status, len(deps))
	}
}
func TestDuplicateStartReturnsSameExecution(t *testing.T) {
	f := newFixture(t, nil)
	a := start(t, f, nil)
	b := start(t, f, nil)
	if a.ID != b.ID {
		t.Fatal("duplicate execution")
	}
}
func TestStartFailureRollsBackRelease(t *testing.T) {
	f := newFixture(t, nil)
	f.db.Exec("CREATE TRIGGER reject_execution BEFORE INSERT ON workflow_executions BEGIN SELECT RAISE(ABORT, 'injected'); END")
	if _, e := f.svc.Start(f.ctx, "r", workflowrun.StartInput{}); e == nil {
		t.Fatal("expected injected failure")
	}
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status != release.ReleaseApproved {
		t.Fatalf("orphan status %s", r.Status)
	}
}
func TestCancelSynchronizesReleaseAndStopsExecution(t *testing.T) {
	f := newFixture(t, nil)
	x := start(t, f, nil)
	if _, e := f.svc.Cancel(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	tick(t, f)
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status != release.ReleaseCancelled {
		t.Fatalf("status %s", r.Status)
	}
}
func TestApprovalRejectionSynchronizesRelease(t *testing.T) {
	f := newFixture(t, nil)
	d := workflowrun.Definition{Name: "approval", Version: 1, Start: "approval", Steps: map[string]workflowrun.Step{"approval": {Type: "approval", Approvers: []string{"owner@example.com"}, Policy: "all", Next: "done"}, "done": {Type: "complete"}}}
	x := start(t, f, &d)
	tick(t, f)
	x, _ = f.repo.Get(f.ctx, x.ID)
	if _, e := f.svc.Approve(f.ctx, x.ID, x.Approvals[0].ID, "rejected"); e != nil {
		t.Fatal(e)
	}
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status != release.ReleaseFailed {
		t.Fatalf("status %s", r.Status)
	}
}
func TestStaleClaimCannotAdvance(t *testing.T) {
	f := newFixture(t, nil)
	x := start(t, f, nil)
	old, ok, e := f.repo.ClaimNext(f.ctx)
	if e != nil || !ok {
		t.Fatal(e)
	}
	f.db.Model(&gormdb.WorkflowExecutionModel{}).Where("id = ?", x.ID).Update("wake_up_at", time.Now().Add(-time.Minute))
	_, ok, e = f.repo.ClaimNext(f.ctx)
	if e != nil || !ok {
		t.Fatal(e)
	}
	if e = f.repo.Advance(f.ctx, old, workflowrun.StepExecution{}, workflowrun.Event{ExecutionID: x.ID, Type: "stale"}); e == nil {
		t.Fatal("stale worker advanced")
	}
}

func TestLongDeploymentRenewsLease(t *testing.T) {
	entered := make(chan struct{})
	finish := make(chan struct{})
	f := newFixture(t, deployFunc(func(ctx context.Context, j executor.DeployJob) (*executor.TriggerResult, error) {
		close(entered)
		select {
		case <-finish:
			return &executor.TriggerResult{Status: "success"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	f.repo = gormdb.NewWorkflowRunRepository(f.db, 80*time.Millisecond)
	f.svc.repo = f.repo
	f.svc.leaseInterval = 10 * time.Millisecond
	start(t, f, nil)
	done := make(chan error, 1)
	go func() { done <- f.svc.Tick(f.ctx) }()
	<-entered
	time.Sleep(180 * time.Millisecond)
	if _, ok, e := f.repo.ClaimNext(f.ctx); e != nil || ok {
		close(finish)
		<-done
		t.Fatalf("live lease reclaimed: %v %v", ok, e)
	}
	close(finish)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func TestCancelInterruptsInFlightDeployment(t *testing.T) {
	entered := make(chan struct{})
	f := newFixture(t, deployFunc(func(ctx context.Context, j executor.DeployJob) (*executor.TriggerResult, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	f.svc.leaseInterval = 10 * time.Millisecond
	x := start(t, f, nil)
	done := make(chan error, 1)
	go func() { done <- f.svc.Tick(f.ctx) }()
	<-entered
	if _, e := f.svc.Cancel(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt executor")
	}
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status != "cancelled" {
		t.Fatal(r.Status)
	}
	deps, _ := f.delivery.ListDeployments(f.ctx, "p")
	if len(deps) != 1 || deps[0].Status != "cancelled" {
		t.Fatalf("%+v", deps)
	}
}
func TestRecoveryReusesOperationAfterUnknownOutcome(t *testing.T) {
	keys := []string{}
	f := newFixture(t, deployFunc(func(ctx context.Context, j executor.DeployJob) (*executor.TriggerResult, error) {
		keys = append(keys, j.IdempotencyKey)
		if len(keys) == 1 {
			return nil, context.DeadlineExceeded
		}
		return &executor.TriggerResult{Status: "success"}, nil
	}))
	d := workflowrun.DefaultReleaseDefinition()
	step := d.Steps[d.Start]
	step.MaxAttempts = 1
	d.Steps[d.Start] = step
	x := start(t, f, &d)
	tick(t, f)
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status != "failed" {
		t.Fatal(r.Status)
	}
	if _, e := f.svc.Retry(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	tick(t, f)
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("keys=%v", keys)
	}
	deps, _ := f.delivery.ListDeployments(f.ctx, "p")
	if len(deps) != 1 || deps[0].Status != "success" {
		t.Fatalf("%+v", deps)
	}
}
func TestRecoverySkipsSuccessfulServices(t *testing.T) {
	calls := map[string]int{}
	f := newFixture(t, deployFunc(func(ctx context.Context, j executor.DeployJob) (*executor.TriggerResult, error) {
		calls[j.ServiceID]++
		status := "success"
		if j.ServiceID == "rs2" && calls[j.ServiceID] == 1 {
			status = "failed"
		}
		return &executor.TriggerResult{Status: status}, nil
	}))
	f.db.Create(&gormdb.ReleaseServiceModel{ID: "rs2", ReleaseID: "r", ProjectServiceID: "s2", DeployTarget: "portainer", DeployConfig: gormdb.JSONMap{}, Status: "pending", OrderIndex: 2})
	d := workflowrun.DefaultReleaseDefinition()
	step := d.Steps[d.Start]
	step.MaxAttempts = 1
	d.Steps[d.Start] = step
	x := start(t, f, &d)
	tick(t, f)
	if _, e := f.svc.Retry(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	tick(t, f)
	if calls["rs"] != 1 || calls["rs2"] != 2 {
		t.Fatalf("calls=%v", calls)
	}
}
func TestExpiredWorkerCannotPersistDeployment(t *testing.T) {
	f := newFixture(t, nil)
	x := start(t, f, nil)
	claim, _, _ := f.repo.ClaimNext(f.ctx)
	dep, e := f.repo.BeginDeployment(f.ctx, claim, delivery.Deployment{ProjectID: "p", ProjectServiceID: "s", ReleaseID: "r", ArtifactID: "a", Environment: "stg", Source: "release"})
	if e != nil {
		t.Fatal(e)
	}
	f.db.Model(&gormdb.WorkflowExecutionModel{}).Where("id = ?", x.ID).Update("wake_up_at", time.Now().Add(-time.Minute))
	if e = f.repo.RenewLease(f.ctx, claim); e == nil {
		t.Fatal("expired lease revived")
	}
	dep.Status = "success"
	if e = f.repo.FinishDeployment(f.ctx, claim, dep); e == nil {
		t.Fatal("expired worker persisted result")
	}
	recovered, ok, e := f.repo.ClaimNext(f.ctx)
	if e != nil || !ok {
		t.Fatal(e)
	}
	retry, e := f.repo.BeginDeployment(f.ctx, recovered, dep)
	if e != nil || retry.ID != dep.ID {
		t.Fatalf("recovery lost identity: %+v %v", retry, e)
	}
}
func TestNonTerminalExecutorResultIsNotSuccess(t *testing.T) {
	f := newFixture(t, deployFunc(func(context.Context, executor.DeployJob) (*executor.TriggerResult, error) {
		return &executor.TriggerResult{Status: "running"}, nil
	}))
	d := workflowrun.DefaultReleaseDefinition()
	s := d.Steps[d.Start]
	s.MaxAttempts = 1
	d.Steps[d.Start] = s
	start(t, f, &d)
	tick(t, f)
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status == "success" {
		t.Fatal("accepted deployment treated as success")
	}
	deps, _ := f.delivery.ListDeployments(f.ctx, "p")
	if deps[0].Status != "running" {
		t.Fatal(deps[0].Status)
	}
}

func TestPublicCommandsUseOneExecution(t *testing.T) {
	f := newFixture(t, nil)
	rs := releaseapp.New(f.releases, f.tasks, gormdb.NewServiceCatalogRepository(f.db), nil, f.delivery, authorization.New(gormdb.NewProjectRepository(f.db)))
	rs.SetWorkflowRunner(f.svc)
	x, e := rs.Publish(f.ctx, "r")
	if e != nil {
		t.Fatal(e)
	}
	y, e := rs.Publish(f.ctx, "r")
	if e != nil {
		t.Fatal(e)
	}
	if x.ID != y.ID {
		t.Fatal("duplicate execution")
	}
	var n int64
	f.db.Model(&gormdb.ReleaseJobModel{}).Count(&n)
	if n != 0 {
		t.Fatal("legacy job created")
	}
	if _, _, e := rs.SetStatus(f.ctx, "r", release.ReleaseCancelled); e != nil {
		t.Fatal(e)
	}
	current, _ := f.repo.GetByRelease(f.ctx, "r")
	if current.Phase != workflowrun.PhaseCancelled {
		t.Fatal(current.Phase)
	}
}

func TestLegacyPendingJobIsExecutedByWorkflowEngine(t *testing.T) {
	f := newFixture(t, nil)
	f.db.Model(&gormdb.ReleaseModel{}).Where("id = 'r'").Update("status", "queued")
	f.db.Create(&gormdb.ReleaseJobModel{ID: "old-job", ReleaseID: "r", Status: "pending"})
	for i := 0; i < 4; i++ {
		tick(t, f)
	}
	r, _ := f.releases.Get(f.ctx, "r")
	if r.Status != "success" {
		t.Fatal(r.Status)
	}
	job, _ := f.releases.GetJob(f.ctx, "old-job")
	if job.Status != "success" {
		t.Fatal(job.Status)
	}
	task, _ := f.tasks.Get(f.ctx, "t")
	if task.Status != "published" {
		t.Fatal(task.Status)
	}
}
func TestLegacyUncertainJobRequiresExplicitRetry(t *testing.T) {
	calls := 0
	f := newFixture(t, deployFunc(func(context.Context, executor.DeployJob) (*executor.TriggerResult, error) {
		calls++
		return &executor.TriggerResult{Status: "success"}, nil
	}))
	f.db.Model(&gormdb.ReleaseModel{}).Where("id = 'r'").Update("status", "running")
	expired := time.Now().Add(-time.Minute)
	f.db.Create(&gormdb.ReleaseJobModel{ID: "old-job", ReleaseID: "r", Status: "running", LeaseUntil: &expired})
	tick(t, f)
	x, e := f.repo.GetByRelease(f.ctx, "r")
	if e != nil {
		t.Fatal(e)
	}
	if calls != 0 || x.Phase != "waiting_retry" {
		t.Fatalf("calls=%d phase=%s", calls, x.Phase)
	}
}
func TestCompletionFailureIsAtomic(t *testing.T) {
	f := newFixture(t, nil)
	x := start(t, f, nil)
	tick(t, f)
	tick(t, f)
	f.db.Exec("CREATE TRIGGER fail_task_event BEFORE INSERT ON task_events BEGIN SELECT RAISE(ABORT, 'injected'); END")
	if e := f.svc.Tick(f.ctx); e == nil {
		t.Fatal("expected error")
	}
	r, _ := f.releases.Get(f.ctx, "r")
	task, _ := f.tasks.Get(f.ctx, "t")
	x, _ = f.repo.Get(f.ctx, x.ID)
	if r.Status == "success" || task.Status == "published" || x.Phase == "completed" {
		t.Fatalf("partial commit: %s %s %s", r.Status, task.Status, x.Phase)
	}
}

func TestDuplicateServiceCannotBeReportedAsDeployed(t *testing.T) {
	calls := 0
	f := newFixture(t, deployFunc(func(context.Context, executor.DeployJob) (*executor.TriggerResult, error) {
		calls++
		return &executor.TriggerResult{Status: "success"}, nil
	}))
	f.db.Create(&gormdb.ReleaseServiceModel{ID: "rs2", ReleaseID: "r", ProjectServiceID: "s", ArtifactID: "different", DeployTarget: "portainer", DeployConfig: gormdb.JSONMap{}, Status: "pending"})
	d := workflowrun.DefaultReleaseDefinition()
	step := d.Steps[d.Start]
	step.MaxAttempts = 1
	d.Steps[d.Start] = step
	start(t, f, &d)
	tick(t, f)
	if calls != 0 {
		t.Fatalf("ambiguous release invoked executor %d times", calls)
	}
}
func TestRecoveredLegacyJobProjection(t *testing.T) {
	f := newFixture(t, nil)
	f.db.Model(&gormdb.ReleaseModel{}).Where("id = 'r'").Update("status", "running")
	expired := time.Now().Add(-time.Minute)
	f.db.Create(&gormdb.ReleaseJobModel{ID: "old-job", ReleaseID: "r", Status: "running", LeaseUntil: &expired})
	tick(t, f)
	x, _ := f.repo.GetByRelease(f.ctx, "r")
	if _, e := f.svc.Retry(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	job, _ := f.releases.GetJob(f.ctx, "old-job")
	if job.Status != "running" {
		t.Fatalf("retried job=%s", job.Status)
	}
	tick(t, f)
	tick(t, f)
	tick(t, f)
	job, _ = f.releases.GetJob(f.ctx, "old-job")
	if job.Status != "success" {
		t.Fatal(job.Status)
	}
}

func TestRecoveredSuccessfulOperationIsNotTriggeredAgain(t *testing.T) {
	calls := 0
	f := newFixture(t, deployFunc(func(context.Context, executor.DeployJob) (*executor.TriggerResult, error) {
		calls++
		return &executor.TriggerResult{Status: "success"}, nil
	}))
	x := start(t, f, nil)
	claim, _, _ := f.repo.ClaimNext(f.ctx)
	dep, e := f.repo.BeginDeployment(f.ctx, claim, delivery.Deployment{ProjectID: "p", ProjectServiceID: "s", ReleaseID: "r", ArtifactID: "a", Environment: "stg", Source: "release"})
	if e != nil {
		t.Fatal(e)
	}
	dep.Status = "success"
	now := time.Now()
	dep.FinishedAt = &now
	if e = f.repo.FinishDeployment(f.ctx, claim, dep); e != nil {
		t.Fatal(e)
	}
	// Process died after saving the deployment, before advancing the step.
	f.db.Model(&gormdb.WorkflowExecutionModel{}).Where("id = ?", x.ID).Update("wake_up_at", time.Now().Add(-time.Minute))
	tick(t, f)
	if calls != 0 {
		t.Fatal("completed operation triggered again")
	}
	x, _ = f.repo.Get(f.ctx, x.ID)
	if x.CurrentStepID != "publish_prod" {
		t.Fatal(x.CurrentStepID)
	}
}
func TestCancelledWorkerCannotWriteLateSuccess(t *testing.T) {
	entered := make(chan struct{})
	finish := make(chan struct{})
	f := newFixture(t, deployFunc(func(ctx context.Context, j executor.DeployJob) (*executor.TriggerResult, error) {
		close(entered)
		<-finish
		return &executor.TriggerResult{Status: "success"}, nil
	}))
	x := start(t, f, nil)
	done := make(chan error, 1)
	go func() { done <- f.svc.Tick(f.ctx) }()
	<-entered
	if _, e := f.svc.Cancel(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	close(finish)
	if e := <-done; e == nil {
		t.Fatal("late result accepted")
	}
	r, _ := f.releases.Get(f.ctx, "r")
	task, _ := f.tasks.Get(f.ctx, "t")
	if r.Status != "cancelled" || task.Status != "publishing" {
		t.Fatalf("%s %s", r.Status, task.Status)
	}
}
func TestWorkerDashboardIncludesWorkflowExecutions(t *testing.T) {
	f := newFixture(t, nil)
	x := start(t, f, nil)
	workers := gormdb.NewWorkerRepository(f.db, 1, time.Second)
	jobs, e := workers.ListJobs(f.ctx, 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 1 || jobs[0].ID != x.ID {
		t.Fatalf("missing execution: %+v", jobs)
	}
	stats, e := workers.QueueStats(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	if stats.Running != 1 {
		t.Fatalf("missing running execution %+v", stats)
	}
}

func TestOldUntrackedExecutionIsQuarantinedBeforeTrigger(t *testing.T) {
	calls := 0
	f := newFixture(t, deployFunc(func(context.Context, executor.DeployJob) (*executor.TriggerResult, error) {
		calls++
		return &executor.TriggerResult{Status: "success"}, nil
	}))
	x := start(t, f, nil)
	f.db.Model(&gormdb.WorkflowExecutionModel{}).Where("id = ?", x.ID).Update("context", gormdb.JSONMap{})
	tick(t, f)
	x, _ = f.repo.Get(f.ctx, x.ID)
	if calls != 0 || x.Phase != "waiting_retry" {
		t.Fatalf("untracked execution replayed: calls=%d phase=%s", calls, x.Phase)
	}
	if _, e := f.svc.Retry(f.ctx, x.ID); e != nil {
		t.Fatal(e)
	}
	tick(t, f)
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestReleaseDetailShowsPublishedLinkedTasks(t *testing.T) {
	f := newFixture(t, nil)
	start(t, f, nil)
	tick(t, f)
	tick(t, f)
	tick(t, f)
	r, e := f.releases.GetFull(f.ctx, "r")
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Tasks) != 1 || r.Tasks[0].Status != "published" {
		t.Fatalf("stale release task %+v", r.Tasks)
	}
}
