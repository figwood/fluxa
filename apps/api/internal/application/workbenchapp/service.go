package workbenchapp

import (
	"context"
	"sort"
	"strings"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/shared"
)

const previewLimit = 5

type projectLister interface {
	List(context.Context) ([]project.Project, error)
}

type taskLister interface {
	List(context.Context, string, task.Status) ([]task.Task, error)
}

type releaseLister interface {
	List(context.Context, string) ([]release.Release, error)
}

type Counts struct {
	MyOpenTasks      int `json:"my_open_tasks"`
	PendingApprovals int `json:"pending_approvals"`
	ActiveReleases   int `json:"active_releases"`
	FailedReleases   int `json:"failed_releases"`
}

type Workbench struct {
	Counts           Counts            `json:"counts"`
	MyTasks          []task.Task       `json:"my_tasks"`
	PendingApprovals []release.Release `json:"pending_approvals"`
	ActiveReleases   []release.Release `json:"active_releases"`
	FailedReleases   []release.Release `json:"failed_releases"`
}

type Service struct {
	projects projectLister
	tasks    taskLister
	releases releaseLister
}

func New(projects projectLister, tasks taskLister, releases releaseLister) *Service {
	return &Service{projects: projects, tasks: tasks, releases: releases}
}

func (s *Service) Get(ctx context.Context, projectID string) (Workbench, error) {
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return Workbench{}, err
	}
	projects, err := s.projects.List(ctx)
	if err != nil {
		return Workbench{}, err
	}
	roles := make(map[string]string, len(projects))
	for _, item := range projects {
		roles[item.ID] = item.CurrentUserRole
	}
	projectID = strings.TrimSpace(projectID)
	if projectID != "" {
		if _, ok := roles[projectID]; !ok {
			return Workbench{}, shared.ErrForbidden
		}
	}

	tasks, err := s.tasks.List(ctx, projectID, "")
	if err != nil {
		return Workbench{}, err
	}
	releases, err := s.releases.List(ctx, projectID)
	if err != nil {
		return Workbench{}, err
	}

	result := Workbench{
		MyTasks:          []task.Task{},
		PendingApprovals: []release.Release{},
		ActiveReleases:   []release.Release{},
		FailedReleases:   []release.Release{},
	}
	for _, item := range tasks {
		if item.Assignee == principal.UserName && item.Status != task.StatusDone && item.Status != task.StatusCancelled {
			result.MyTasks = append(result.MyTasks, item)
		}
	}
	for _, item := range releases {
		canLead := roles[item.ProjectID] == authorization.RoleLead || roles[item.ProjectID] == authorization.RoleOwner
		switch item.Status {
		case release.ReleasePendingApproval:
			if canLead {
				result.PendingApprovals = append(result.PendingApprovals, item)
			}
		case release.ReleaseQueued, release.ReleaseRunning, release.ReleasePublishing:
			result.ActiveReleases = append(result.ActiveReleases, item)
		case release.ReleaseFailed:
			if canLead {
				result.FailedReleases = append(result.FailedReleases, item)
			}
		}
	}

	sortTasks(result.MyTasks)
	sortReleases(result.PendingApprovals)
	sortReleases(result.ActiveReleases)
	sortReleases(result.FailedReleases)
	result.Counts = Counts{
		MyOpenTasks: len(result.MyTasks), PendingApprovals: len(result.PendingApprovals),
		ActiveReleases: len(result.ActiveReleases), FailedReleases: len(result.FailedReleases),
	}
	result.MyTasks = firstTasks(result.MyTasks)
	result.PendingApprovals = firstReleases(result.PendingApprovals)
	result.ActiveReleases = firstReleases(result.ActiveReleases)
	result.FailedReleases = firstReleases(result.FailedReleases)
	return result, nil
}

func sortTasks(items []task.Task) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
}

func sortReleases(items []release.Release) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
}

func firstTasks(items []task.Task) []task.Task {
	if len(items) > previewLimit {
		return items[:previewLimit]
	}
	return items
}

func firstReleases(items []release.Release) []release.Release {
	if len(items) > previewLimit {
		return items[:previewLimit]
	}
	return items
}
