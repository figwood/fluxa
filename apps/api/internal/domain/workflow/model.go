package workflow

import (
	"strings"
	"time"
)

type Kind string

const (
	KindTask    Kind = "task"
	KindRelease Kind = "release"
)

type Workflow struct {
	ProjectID     string       `json:"project_id"`
	Kind          Kind         `json:"kind"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	InitialStatus string       `json:"initial_status"`
	Stages        []Stage      `json:"stages"`
	Statuses      []Status     `json:"statuses"`
	Transitions   []Transition `json:"transitions"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Stage struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OrderIndex int    `json:"order_index"`
}

type Status struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	StageID    string `json:"stage_id"`
	Category   string `json:"category"`
	OrderIndex int    `json:"order_index"`
}

type Transition struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Action     string `json:"action"`
	OrderIndex int    `json:"order_index"`
}

type UpdateWorkflowInput struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	InitialStatus string       `json:"initial_status"`
	Stages        []Stage      `json:"stages"`
	Statuses      []Status     `json:"statuses"`
	Transitions   []Transition `json:"transitions"`
}

func (k Kind) Valid() bool {
	return k == KindTask || k == KindRelease
}

func NormalizeStatusID(v string) string {
	return strings.TrimSpace(v)
}

func DefaultWorkflows() []Workflow {
	now := time.Now()
	return []Workflow{
		{
			Kind:          KindTask,
			Name:          "任务工作流",
			Description:   "任务单从待办到完成的基础状态流转",
			InitialStatus: "todo",
			CreatedAt:     now,
			UpdatedAt:     now,
			Stages: []Stage{
				{ID: "backlog", Name: "待处理", OrderIndex: 1},
				{ID: "delivery", Name: "交付中", OrderIndex: 2},
				{ID: "acceptance", Name: "验收", OrderIndex: 3},
				{ID: "closed", Name: "结束", OrderIndex: 4},
			},
			Statuses: []Status{
				{ID: "todo", Name: "待办", StageID: "backlog", Category: "normal", OrderIndex: 1},
				{ID: "in_progress", Name: "处理中", StageID: "delivery", Category: "active", OrderIndex: 2},
				{ID: "publishing", Name: "待发布", StageID: "delivery", Category: "active", OrderIndex: 3},
				{ID: "published", Name: "已发布", StageID: "acceptance", Category: "success", OrderIndex: 4},
				{ID: "done", Name: "完成", StageID: "closed", Category: "terminal", OrderIndex: 5},
				{ID: "cancelled", Name: "已取消", StageID: "closed", Category: "terminal", OrderIndex: 6},
			},
			Transitions: []Transition{
				{ID: "start", Name: "开始处理", FromStatus: "todo", ToStatus: "in_progress", Action: "start", OrderIndex: 1},
				{ID: "ready_to_publish", Name: "进入发布", FromStatus: "in_progress", ToStatus: "publishing", Action: "mark_publishing", OrderIndex: 2},
				{ID: "mark_published", Name: "标记已发布", FromStatus: "publishing", ToStatus: "published", Action: "mark_published", OrderIndex: 3},
				{ID: "finish", Name: "完成", FromStatus: "published", ToStatus: "done", Action: "finish", OrderIndex: 4},
				{ID: "cancel_todo", Name: "取消", FromStatus: "todo", ToStatus: "cancelled", Action: "cancel", OrderIndex: 5},
				{ID: "cancel_progress", Name: "取消", FromStatus: "in_progress", ToStatus: "cancelled", Action: "cancel", OrderIndex: 6},
				{ID: "cancel_publishing", Name: "取消", FromStatus: "publishing", ToStatus: "cancelled", Action: "cancel", OrderIndex: 7},
			},
		},
		{
			Kind:          KindRelease,
			Name:          "发布工作流",
			Description:   "发布单从草稿、审批到 Worker 执行的基础状态流转",
			InitialStatus: "draft",
			CreatedAt:     now,
			UpdatedAt:     now,
			Stages: []Stage{
				{ID: "prepare", Name: "准备", OrderIndex: 1},
				{ID: "approval", Name: "审批", OrderIndex: 2},
				{ID: "execution", Name: "执行", OrderIndex: 3},
				{ID: "closed", Name: "结束", OrderIndex: 4},
			},
			Statuses: []Status{
				{ID: "draft", Name: "草稿", StageID: "prepare", Category: "normal", OrderIndex: 1},
				{ID: "pending_approval", Name: "待审批", StageID: "approval", Category: "warning", OrderIndex: 2},
				{ID: "approved", Name: "已审批", StageID: "approval", Category: "active", OrderIndex: 3},
				{ID: "queued", Name: "已排队", StageID: "execution", Category: "active", OrderIndex: 4},
				{ID: "running", Name: "发布中", StageID: "execution", Category: "active", OrderIndex: 5},
				{ID: "success", Name: "成功", StageID: "closed", Category: "success", OrderIndex: 6},
				{ID: "failed", Name: "失败", StageID: "closed", Category: "terminal", OrderIndex: 7},
				{ID: "cancelled", Name: "已取消", StageID: "closed", Category: "terminal", OrderIndex: 8},
			},
			Transitions: []Transition{
				{ID: "submit", Name: "提交审批", FromStatus: "draft", ToStatus: "pending_approval", Action: "submit", OrderIndex: 1},
				{ID: "approve", Name: "审批通过", FromStatus: "pending_approval", ToStatus: "approved", Action: "approve", OrderIndex: 2},
				{ID: "enqueue", Name: "自动入队", FromStatus: "approved", ToStatus: "queued", Action: "enqueue", OrderIndex: 3},
				{ID: "cancel_draft", Name: "取消", FromStatus: "draft", ToStatus: "cancelled", Action: "cancel", OrderIndex: 4},
				{ID: "cancel_pending", Name: "取消", FromStatus: "pending_approval", ToStatus: "cancelled", Action: "cancel", OrderIndex: 5},
				{ID: "cancel_approved", Name: "取消", FromStatus: "approved", ToStatus: "cancelled", Action: "cancel", OrderIndex: 6},
			},
		},
	}
}
