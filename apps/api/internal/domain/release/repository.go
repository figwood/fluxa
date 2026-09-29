package release

import "context"

type Repository interface {
	Create(ctx context.Context, rel Release) (Release, error)
	CreateWithEvent(ctx context.Context, rel Release, event ReleaseEvent) (Release, error)
	List(ctx context.Context) ([]Release, error)
	Get(ctx context.Context, id string) (Release, error)
	GetFull(ctx context.Context, id string) (Release, error)
	UpdateStatus(ctx context.Context, id string, status ReleaseStatus) (Release, error)
	TransitionStatus(ctx context.Context, id string, from ReleaseStatus, to ReleaseStatus, event ReleaseEvent) (Release, error)
	UpdateService(ctx context.Context, svc ReleaseService) error
	ListServices(ctx context.Context, releaseID string) ([]ReleaseService, error)
	CreateJob(ctx context.Context, job ReleaseJob) (ReleaseJob, error)
	Enqueue(ctx context.Context, id string, allowedFrom []ReleaseStatus, job ReleaseJob, event ReleaseEvent) (ReleaseJob, bool, error)
	ClaimNextJob(ctx context.Context, workerID string) (ReleaseJob, bool, error)
	RenewJobLease(ctx context.Context, jobID, workerID string) error
	ReapExpiredJobs(ctx context.Context) ([]ReleaseJob, error)
	GetJob(ctx context.Context, id string) (ReleaseJob, error)
	GetLatestJob(ctx context.Context, releaseID string) (ReleaseJob, error)
	UpdateJob(ctx context.Context, job ReleaseJob) error
	AddEvent(ctx context.Context, event ReleaseEvent) error
	ListEvents(ctx context.Context, releaseID string) ([]ReleaseEvent, error)
}
