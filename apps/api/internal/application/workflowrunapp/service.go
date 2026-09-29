package workflowrunapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/executor"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/shared"
)

type Service struct {
	repo          workflowrun.Repository
	releases      release.Repository
	delivery      delivery.Repository
	executors     executor.Registry
	access        *authorization.Service
	leaseInterval time.Duration
}

func New(repo workflowrun.Repository, releases release.Repository, deliveryRepo delivery.Repository, executors executor.Registry, access *authorization.Service) *Service {
	return &Service{repo: repo, releases: releases, delivery: deliveryRepo, executors: executors, access: access, leaseInterval: 5 * time.Second}
}

func (s *Service) Start(ctx context.Context, releaseID string, in workflowrun.StartInput) (workflowrun.Execution, error) {
	rel, err := s.releases.GetFull(ctx, releaseID)
	if err != nil {
		return workflowrun.Execution{}, err
	}
	if err = s.access.Require(ctx, rel.ProjectID, authorization.RoleDeveloper); err != nil {
		return workflowrun.Execution{}, err
	}
	if existing, err := s.repo.GetByRelease(ctx, releaseID); err == nil {
		return existing, nil
	} else if !errors.Is(err, shared.ErrNotFound) {
		return workflowrun.Execution{}, err
	}
	if rel.Status != release.ReleaseDraft && rel.Status != release.ReleaseApproved {
		return workflowrun.Execution{}, shared.ErrInvalidTransition
	}
	if rel.Status == release.ReleaseDraft {
		policy, err := s.delivery.GetPolicy(ctx, rel.ProjectID, "prod")
		if err != nil {
			return workflowrun.Execution{}, err
		}
		if policy.Approval {
			return workflowrun.Execution{}, shared.ErrInvalidTransition
		}
	}
	if rel.Status == release.ReleaseApproved || in.Definition != nil {
		if err := s.access.Require(ctx, rel.ProjectID, authorization.RoleLead); err != nil {
			return workflowrun.Execution{}, err
		}
	}
	def := workflowrun.DefaultReleaseDefinition()
	if in.Definition != nil {
		def = *in.Definition
	}
	if err = ValidateDefinition(def); err != nil {
		return workflowrun.Execution{}, err
	}
	now := time.Now()
	item := workflowrun.Execution{ID: shared.NewID("wfexec"), ReleaseID: rel.ID, ProjectID: rel.ProjectID, DefinitionName: def.Name, DefinitionVersion: def.Version, DefinitionSnapshot: def, PublicStatus: "publishing", Phase: workflowrun.PhaseRunning, CurrentStepID: def.Start, Context: in.Context, LockVersion: 1, CreatedAt: now, UpdatedAt: now}
	if item.Context == nil {
		item.Context = map[string]any{}
	}
	item.Context["release"] = releaseContext(rel)
	item.Context["deployment_tracking_v1"] = true

	return s.repo.Create(ctx, item)
}

func (s *Service) GetByRelease(ctx context.Context, releaseID string) (workflowrun.Execution, error) {
	rel, err := s.releases.Get(ctx, releaseID)
	if err != nil {
		return workflowrun.Execution{}, err
	}
	if err = s.access.Require(ctx, rel.ProjectID, authorization.RoleDeveloper); err != nil {
		return workflowrun.Execution{}, err
	}
	return s.repo.GetByRelease(ctx, releaseID)
}
func (s *Service) Get(ctx context.Context, id string) (workflowrun.Execution, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	if err = s.access.Require(ctx, item.ProjectID, authorization.RoleDeveloper); err != nil {
		return workflowrun.Execution{}, err
	}
	return item, nil
}
func (s *Service) Approve(ctx context.Context, id, taskID, decision string) (workflowrun.Execution, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	if err = s.access.Require(ctx, item.ProjectID, authorization.RoleDeveloper); err != nil {
		return item, err
	}
	p, err := authorization.Principal(ctx)
	if err != nil {
		return item, err
	}
	return s.repo.Approve(ctx, id, taskID, p.UserEmail, decision)
}
func (s *Service) Retry(ctx context.Context, id string) (workflowrun.Execution, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	if err = s.access.Require(ctx, item.ProjectID, authorization.RoleLead); err != nil {
		return item, err
	}
	return s.repo.Retry(ctx, id)
}
func (s *Service) Cancel(ctx context.Context, id string) (workflowrun.Execution, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	if err = s.access.Require(ctx, item.ProjectID, authorization.RoleLead); err != nil {
		return item, err
	}
	p, _ := authorization.Principal(ctx)
	return s.repo.Cancel(ctx, id, p.UserEmail)
}

func ValidateDefinition(def workflowrun.Definition) error {
	if strings.TrimSpace(def.Name) == "" || def.Version < 1 || def.Start == "" || len(def.Steps) == 0 {
		return fmt.Errorf("%w: invalid workflow metadata", shared.ErrInvalidInput)
	}
	if _, ok := def.Steps[def.Start]; !ok {
		return fmt.Errorf("%w: start step not found", shared.ErrInvalidInput)
	}
	for id, step := range def.Steps {
		if id == "" {
			return fmt.Errorf("%w: empty step id", shared.ErrInvalidInput)
		}
		references := []string{}
		switch step.Type {
		case "condition":
			if _, err := compileBool(step.Expression); err != nil {
				return fmt.Errorf("%w: step %s: %v", shared.ErrInvalidInput, id, err)
			}
			references = []string{step.OnTrue, step.OnFalse}
		case "approval":
			if len(step.Approvers) == 0 {
				return fmt.Errorf("%w: step %s has no approvers", shared.ErrInvalidInput, id)
			}
			if step.Policy == "" {
				step.Policy = "all"
			}
			if step.Policy != "all" && step.Policy != "any" && step.Policy != "quorum" {
				return fmt.Errorf("%w: invalid approval policy", shared.ErrInvalidInput)
			}
			if step.Policy == "quorum" && (step.Quorum < 1 || step.Quorum > len(step.Approvers)) {
				return fmt.Errorf("%w: invalid quorum", shared.ErrInvalidInput)
			}
			references = []string{step.Next}
		case "wait_until":
			if _, err := evalTimestamp(step.Until, map[string]any{"now": time.Now()}); err != nil && !strings.Contains(step.Until, ".") {
				return fmt.Errorf("%w: step %s: %v", shared.ErrInvalidInput, id, err)
			}
			references = []string{step.Next}
		case "action":
			if step.Action != "publish_environment" && step.Action != "send_message" && step.Action != "assign_issue" && step.Action != "rollback_environment" {
				return fmt.Errorf("%w: unknown action %s", shared.ErrInvalidInput, step.Action)
			}
			references = []string{step.OnSuccess}
			if step.OnFailure != "" {
				references = append(references, step.OnFailure)
			}
		case "complete":
		default:
			return fmt.Errorf("%w: unknown step type %s", shared.ErrInvalidInput, step.Type)
		}
		for _, next := range references {
			if next == "" {
				return fmt.Errorf("%w: step %s has empty target", shared.ErrInvalidInput, id)
			}
			if _, ok := def.Steps[next]; !ok {
				return fmt.Errorf("%w: step %s targets missing step %s", shared.ErrInvalidInput, id, next)
			}
		}
	}
	return nil
}

func (s *Service) Tick(ctx context.Context) error {
	if err := s.repo.AdoptLegacy(ctx); err != nil {
		return err
	}
	item, ok, err := s.repo.ClaimNext(ctx)
	if err != nil || !ok {
		return err
	}
	return s.withLease(ctx, item)
}

func (s *Service) withLease(ctx context.Context, item workflowrun.Execution) error {
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(s.leaseInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-workCtx.Done():
				return
			case <-ticker.C:
				renewCtx, stop := context.WithTimeout(workCtx, s.leaseInterval)
				err := s.repo.RenewLease(renewCtx, item)
				stop()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	err := s.executeStep(workCtx, item)
	close(done)
	<-stopped
	return err
}
func (s *Service) executeStep(ctx context.Context, item workflowrun.Execution) error {
	step, ok := item.DefinitionSnapshot.Steps[item.CurrentStepID]
	if !ok {
		return s.markTerminalFailure(ctx, item, "current step missing")
	}
	now := time.Now()
	attempt := 1
	for _, old := range item.Steps {
		if old.StepID == item.CurrentStepID && old.Attempt >= attempt {
			attempt = old.Attempt + 1
		}
	}
	run := workflowrun.StepExecution{ID: shared.NewID("wfstep"), ExecutionID: item.ID, StepID: item.CurrentStepID, Attempt: attempt, Status: workflowrun.StepRunning, IdempotencyKey: fmt.Sprintf("%s:%s:%d", item.ID, item.CurrentStepID, attempt), Input: step.Params, Output: map[string]any{}, StartedAt: now}
	event := workflowrun.Event{ID: shared.NewID("wfevent"), ExecutionID: item.ID, Type: "step.completed", Payload: map[string]any{"step_id": item.CurrentStepID}, CreatedAt: now}
	switch step.Type {
	case "condition":
		value, err := s.evalBool(item, step.Expression)
		if err != nil {
			return s.failStep(ctx, item, step, run, err)
		}
		if value {
			item.CurrentStepID = step.OnTrue
		} else {
			item.CurrentStepID = step.OnFalse
		}
		finishStep(&run, nil)
		item.WakeUpAt = nil
		return s.repo.Advance(ctx, item, run, event)
	case "approval":
		tasks := make([]workflowrun.ApprovalTask, 0, len(step.Approvers))
		for _, approver := range step.Approvers {
			tasks = append(tasks, workflowrun.ApprovalTask{ID: shared.NewID("approval"), ExecutionID: item.ID, StepID: item.CurrentStepID, Approver: approver, Decision: "pending", CreatedAt: now})
		}
		item.Phase = workflowrun.PhaseWaitingApproval
		item.WakeUpAt = nil
		finishStep(&run, nil)
		event.Type = "approval.requested"
		return s.repo.Pause(ctx, item, run, tasks, event)
	case "wait_until":
		until, err := s.evalTime(item, step.Until)
		if err != nil {
			return s.failStep(ctx, item, step, run, err)
		}
		if until.After(now) {
			item.Phase = workflowrun.PhaseWaitingTime
			item.WakeUpAt = &until
			run.Status = workflowrun.StepPending
			return s.repo.Pause(ctx, item, run, nil, workflowrun.Event{ID: shared.NewID("wfevent"), ExecutionID: item.ID, Type: "timer.scheduled", Payload: map[string]any{"wake_up_at": until}, CreatedAt: now})
		}
		item.CurrentStepID = step.Next
		finishStep(&run, nil)
		item.WakeUpAt = nil
		return s.repo.Advance(ctx, item, run, event)
	case "action":
		output, err := s.executeAction(ctx, item, step)
		if err != nil {
			return s.failStep(ctx, item, step, run, err)
		}
		run.Output = output
		finishStep(&run, nil)
		item.CurrentStepID = step.OnSuccess
		item.WakeUpAt = nil
		return s.repo.Advance(ctx, item, run, event)
	case "complete":
		finishStep(&run, nil)
		item.Phase = workflowrun.PhaseCompleted
		item.PublicStatus = "success"
		item.WakeUpAt = nil
		item.CompletedAt = &now
		event.Type = "execution.completed"
		return s.repo.Complete(ctx, item, run, event)
	default:
		return s.failStep(ctx, item, step, run, fmt.Errorf("unsupported step type %s", step.Type))
	}
}

func expressionVars(item workflowrun.Execution) map[string]any {
	vars := map[string]any{"release": item.Context["release"], "project": map[string]any{}, "services": []any{}, "actor": map[string]any{}, "approvals": map[string]any{}, "now": time.Now()}
	for k, v := range item.Context {
		vars[k] = v
	}
	return vars
}
func (s *Service) evalBool(item workflowrun.Execution, expression string) (bool, error) {
	p, e := compileBool(expression)
	if e != nil {
		return false, e
	}
	return p.Eval(expressionVars(item))
}
func (s *Service) evalTime(item workflowrun.Execution, expression string) (time.Time, error) {
	return evalTimestamp(expression, expressionVars(item))
}

func (s *Service) failStep(ctx context.Context, item workflowrun.Execution, step workflowrun.Step, run workflowrun.StepExecution, cause error) error {
	finishStep(&run, cause)
	item.LastError = cause.Error()
	max := step.MaxAttempts
	if max < 1 {
		max = 1
	}
	if run.Attempt < max {
		delay := step.RetryDelaySec
		if delay < 1 {
			delay = 5
		}
		next := time.Now().Add(time.Duration(delay) * time.Second)
		item.Phase = workflowrun.PhaseRunning
		return s.repo.FailAttempt(ctx, item, run, next, workflowrun.Event{ID: shared.NewID("wfevent"), ExecutionID: item.ID, Type: "step.retry_scheduled", Payload: map[string]any{"step_id": item.CurrentStepID, "error": cause.Error(), "attempt": run.Attempt}, CreatedAt: time.Now()})
	}
	if step.OnFailure != "" {
		item.CurrentStepID = step.OnFailure
		item.Phase = workflowrun.PhaseRunning
		item.WakeUpAt = nil
		return s.repo.Advance(ctx, item, run, workflowrun.Event{ID: shared.NewID("wfevent"), ExecutionID: item.ID, Type: "step.failed", Payload: map[string]any{"error": cause.Error()}, CreatedAt: time.Now()})
	}
	item.Phase = workflowrun.PhaseWaitingRetry
	item.PublicStatus = "failed"
	item.WakeUpAt = nil
	return s.repo.Advance(ctx, item, run, workflowrun.Event{ID: shared.NewID("wfevent"), ExecutionID: item.ID, Type: "execution.waiting_retry", Payload: map[string]any{"error": cause.Error()}, CreatedAt: time.Now()})
}
func (s *Service) markTerminalFailure(ctx context.Context, item workflowrun.Execution, message string) error {
	item.Phase = workflowrun.PhaseFailed
	item.PublicStatus = "failed"
	item.LastError = message
	item.WakeUpAt = nil
	return s.repo.Advance(ctx, item, workflowrun.StepExecution{}, workflowrun.Event{ID: shared.NewID("wfevent"), ExecutionID: item.ID, Type: "execution.failed", Payload: map[string]any{"error": message}, CreatedAt: time.Now()})
}
func finishStep(run *workflowrun.StepExecution, err error) {
	now := time.Now()
	run.FinishedAt = &now
	if err != nil {
		run.Status = workflowrun.StepFailed
		run.Error = err.Error()
	} else {
		run.Status = workflowrun.StepSuccess
	}
}
func releaseContext(rel release.Release) map[string]any {
	services := make([]map[string]any, 0, len(rel.Services))
	for _, svc := range rel.Services {
		services = append(services, map[string]any{"id": svc.ID, "key": svc.ServiceKey, "name": svc.DisplayName})
	}
	return map[string]any{"id": rel.ID, "project_id": rel.ProjectID, "status": string(rel.Status), "services": services, "details": rel.Details}
}

func (s *Service) executeAction(ctx context.Context, item workflowrun.Execution, step workflowrun.Step) (map[string]any, error) {
	switch step.Action {
	case "publish_environment":
		env, _ := step.Params["environment"].(string)
		if env != "stg" && env != "prod" {
			return nil, fmt.Errorf("invalid environment")
		}
		return s.publishEnvironment(ctx, item, env)
	case "send_message", "assign_issue", "rollback_environment":
		return map[string]any{"accepted": true}, nil
	default:
		return nil, fmt.Errorf("action %s not registered", step.Action)
	}
}

func (s *Service) publishEnvironment(ctx context.Context, item workflowrun.Execution, environment string) (map[string]any, error) {
	rel, err := s.releases.GetFull(ctx, item.ReleaseID)
	if err != nil {
		return nil, err
	}
	seenServices := map[string]bool{}
	for _, svc := range rel.Services {
		if seenServices[svc.ProjectServiceID] {
			return nil, fmt.Errorf("duplicate release service %s", svc.ProjectServiceID)
		}
		seenServices[svc.ProjectServiceID] = true
	}
	taskIDs := []string{}
	for _, t := range rel.Tasks {
		taskIDs = append(taskIDs, t.TaskID)
	}
	for _, svc := range rel.Services {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		artifact := delivery.Artifact{ID: svc.ArtifactID, ImageDigest: svc.Ref}
		if svc.ArtifactID != "" {
			artifact, err = s.delivery.GetArtifact(ctx, svc.ArtifactID)
			if err != nil {
				return nil, err
			}
		}
		exec, ok := s.executors.Get(svc.DeployTarget)
		if !ok {
			return nil, fmt.Errorf("executor %q not registered", svc.DeployTarget)
		}
		dep, err := s.repo.BeginDeployment(ctx, item, delivery.Deployment{ProjectID: rel.ProjectID, ProjectServiceID: svc.ProjectServiceID, ArtifactID: artifact.ID, ReleaseID: rel.ID, Environment: environment, Source: "release"})
		if err != nil {
			return nil, err
		}
		if dep.Status == "success" {
			continue
		}
		if dep.Status != "running" {
			return nil, shared.ErrInvalidTransition
		}
		result, err := exec.Trigger(ctx, executor.DeployJob{IdempotencyKey: dep.ID, ReleaseID: rel.ID, ServiceID: svc.ID, TaskIDs: taskIDs, Ref: artifact.ImageDigest, Environment: environment, Config: svc.DeployConfig})
		// A transport error or non-terminal result is ambiguous. Preserve the operation
		// so a retry reconciles the SAME key, rather than launching another deployment.
		if err != nil {
			return nil, err
		}
		if result == nil || (result.Status != "success" && result.Status != "failed") {
			return nil, fmt.Errorf("deployment outcome not terminal; retry will reconcile operation %s", dep.ID)
		}
		now := time.Now()
		dep.Status = result.Status
		dep.ExternalID = result.ExternalID
		dep.ExternalURL = result.ExternalURL
		dep.Message = result.Message
		dep.FinishedAt = &now
		if err := s.repo.FinishDeployment(ctx, item, dep); err != nil {
			return nil, err
		}
		if dep.Status == "failed" {
			return nil, fmt.Errorf("deployment failed: %s", dep.Message)
		}
	}
	return map[string]any{"environment": environment, "services": len(rel.Services)}, nil
}
