package mock

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"fluxa-api/internal/domain/executor"
	"fluxa-api/internal/shared"
)

type Registry struct {
	items map[string]executor.Executor
}

func NewRegistry() *Registry {
	e := Executor{}
	return &Registry{items: map[string]executor.Executor{
		"jenkins":   e,
		"portainer": e,
	}}
}

func (r *Registry) Get(target string) (executor.Executor, bool) {
	item, ok := r.items[strings.ToLower(strings.TrimSpace(target))]
	return item, ok
}

type Executor struct{}

func (Executor) Trigger(ctx context.Context, job executor.DeployJob) (*executor.TriggerResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(400 * time.Millisecond):
	}
	if result, ok := job.Config["mock_result"].(string); ok && strings.EqualFold(result, "failed") {
		return &executor.TriggerResult{Status: "failed", Message: fmt.Sprintf("mock deployment failed for service %s", job.ServiceID)}, nil
	}
	id := shared.NewID("ext")
	if job.IdempotencyKey != "" {
		id = fmt.Sprintf("ext_%x", sha256.Sum256([]byte(job.IdempotencyKey)))
	}
	return &executor.TriggerResult{
		ExternalID:  id,
		ExternalURL: fmt.Sprintf("https://mock.fluxa.local/releases/%s/services/%s", job.ReleaseID, job.ServiceID),
		Status:      "success",
		Message:     "mock deployment completed",
	}, nil
}
