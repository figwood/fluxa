package executor

import "context"

// IdempotencyKey identifies one durable external operation. Executors must return
// the same operation/result when called again with the same key after a restart.
// Errors mean an unknown outcome; only Status="failed" permits a new attempt.
type DeployJob struct {
	IdempotencyKey string
	ReleaseID      string
	ServiceID      string
	TaskIDs        []string
	Ref            string
	Environment    string
	Config         map[string]any
}

type TriggerResult struct {
	ExternalID  string `json:"external_id"`
	ExternalURL string `json:"external_url"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type Executor interface {
	Trigger(ctx context.Context, job DeployJob) (*TriggerResult, error)
}

type Registry interface {
	Get(target string) (Executor, bool)
}
