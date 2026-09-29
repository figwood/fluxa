package workflowrun

import (
	"context"
	"fluxa-api/internal/domain/delivery"
	"time"
)

type Repository interface {
	Create(context.Context, Execution) (Execution, error)
	Get(context.Context, string) (Execution, error)
	GetByRelease(context.Context, string) (Execution, error)
	AdoptLegacy(context.Context) error
	ClaimNext(context.Context) (Execution, bool, error)
	RenewLease(context.Context, Execution) error
	BeginDeployment(context.Context, Execution, delivery.Deployment) (delivery.Deployment, error)
	FinishDeployment(context.Context, Execution, delivery.Deployment) error
	Advance(context.Context, Execution, StepExecution, Event) error
	Pause(context.Context, Execution, StepExecution, []ApprovalTask, Event) error
	Complete(context.Context, Execution, StepExecution, Event) error
	FailAttempt(context.Context, Execution, StepExecution, time.Time, Event) error
	Approve(context.Context, string, string, string, string) (Execution, error)
	Retry(context.Context, string) (Execution, error)
	Cancel(context.Context, string, string) (Execution, error)
}
