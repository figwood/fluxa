export type ApiResponse<T> = {
  code: number;
  message: string;
  data?: T;
};

export type Project = {
  id: string;
  name: string;
  key: string;
  description: string;
  current_user_role: 'owner' | 'lead' | 'developer';
};

export type ProjectMember = {
  id: number;
  project_id: string;
  user_id: number;
  user_name: string;
  user_email: string;
  role_id: number;
  role_name: 'owner' | 'lead' | 'developer';
};

export type ProjectMemberList = {
  items: ProjectMember[];
  can_manage: boolean;
};

export type ProjectMemberCandidate = {
  user_id: number;
  user_name: string;
  user_email: string;
};

export type GitlabRepository = {
  id: string;
  gitlab_project_id: number;
  name: string;
  path: string;
  url: string;
  default_branch: string;
};

export type RemoteGitlabProject = {
  id: number;
  name: string;
  path_with_namespace: string;
  web_url: string;
  default_branch: string;
};

export type ProjectService = {
  id: string;
  project_id: string;
  gitlab_project_id: number;
  service_key: string;
  display_name: string;
  module_path: string;
  image_name: string;
  deploy_target: 'jenkins' | 'portainer';
  deploy_config: Record<string, unknown>;
  status: string;
};

export type Task = {
  id: string;
  project_id: string;
  title: string;
  description: string;
  creator: string;
  assignee: string;
  status: 'todo' | 'in_progress' | 'publishing' | 'published' | 'done' | 'cancelled';
  details: Record<string, unknown>;
  events?: TaskEvent[];
  created_at: string;
  updated_at: string;
};

export type TaskEvent = {
  id: string;
  task_id: string;
  type: string;
  title: string;
  message: string;
  actor: string;
  from_status: string;
  to_status: string;
  created_at: string;
};

export type ReleaseTask = {
  id: string;
  release_id: string;
  task_id: string;
  title: string;
  status: string;
};

export type ReleaseService = {
  id: string;
  release_id: string;
  project_service_id: string;
  artifact_id: string;
  service_key: string;
  display_name: string;
  deploy_target: string;
  deploy_config: Record<string, unknown>;
  ref: string;
  order_index: number;
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped';
  external_id: string;
  external_url: string;
  message: string;
};

export type Release = {
  id: string;
  project_id: string;
  title: string;
  description: string;
  status: 'draft' | 'pending_approval' | 'approved' | 'queued' | 'running' | 'publishing' | 'success' | 'failed' | 'cancelled';
  details: Record<string, unknown>;
  tasks?: ReleaseTask[];
  services?: ReleaseService[];
  events?: ReleaseEvent[];
  created_at: string;
  updated_at: string;
};

export type WorkbenchResponse = {
  counts: {
    my_open_tasks: number;
    pending_approvals: number;
    active_releases: number;
    failed_releases: number;
  };
  my_tasks: Task[];
  pending_approvals: Release[];
  active_releases: Release[];
  failed_releases: Release[];
};

export type Artifact = {
  id: string;
  project_id: string;
  project_service_id: string;
  commit_sha: string;
  ref_type: 'branch' | 'tag' | string;
  ref: string;
  image_repository: string;
  image_tag: string;
  image_digest: string;
  pipeline_id: string;
  pipeline_url: string;
  built_at: string;
  created_at: string;
};

export type Deployment = {
  id: string;
  project_id: string;
  project_service_id: string;
  artifact_id: string;
  release_id?: string;
  environment: string;
  source: string;
  status: string;
  external_url: string;
  message: string;
  created_at: string;
};

export type EnvironmentPolicy = {
  project_id: string;
  environment: 'dev' | 'stg' | 'prod';
  auto_deploy: boolean;
  approval_required: boolean;
  completes_tasks: boolean;
};

export type ReleaseJob = {
  id: string;
  release_id: string;
  status: 'pending' | 'running' | 'success' | 'failed';
  worker_id: string;
  message: string;
  started_at?: string;
  finished_at?: string;
  lease_until?: string;
  heartbeat_at?: string;
  attempt: number;
  created_at: string;
  updated_at: string;
};

export type ReleaseEvent = {
  id: string;
  release_id: string;
  type: string;
  title: string;
  message: string;
  actor: string;
  from_status: string;
  to_status: string;
  job_id: string;
  service_id: string;
  service_name: string;
  created_at: string;
};

export type WorkflowKind = 'task' | 'release';

export type WorkflowStatusConfig = {
  id: string;
  name: string;
  stage_id: string;
  category: 'normal' | 'active' | 'warning' | 'success' | 'terminal' | string;
  order_index: number;
};

export type WorkflowStageConfig = {
  id: string;
  name: string;
  order_index: number;
};

export type WorkflowTransitionConfig = {
  id: string;
  name: string;
  from_status: string;
  to_status: string;
  action: string;
  order_index: number;
};

export type WorkflowConfig = {
  project_id: string;
  kind: WorkflowKind;
  name: string;
  description: string;
  initial_status: string;
  stages: WorkflowStageConfig[];
  statuses: WorkflowStatusConfig[];
  transitions: WorkflowTransitionConfig[];
};

export type ReleaseStatusUpdate = {
  release: Release;
  execution?: WorkflowExecution | null;
};

export type WorkflowApprovalTask = {
  id: string;
  step_id: string;
  approver: string;
  decision: 'pending' | 'approved' | 'rejected';
  decided_at?: string;
};

export type WorkflowExecution = {
  id: string;
  release_id: string;
  public_status: string;
  phase: 'running' | 'waiting_approval' | 'waiting_time' | 'waiting_retry' | 'completed' | 'failed' | 'cancelled';
  current_step_id: string;
  wake_up_at?: string;
  last_error: string;
  approvals?: WorkflowApprovalTask[];
};

export type WorkerRuntimeConfig = {
  desired_replicas: number;
  poll_interval_ms: number;
  mode: string;
};

export type WorkerQueueStats = {
  pending: number;
  running: number;
  success: number;
  failed: number;
};

export type WorkerHeartbeat = {
  id: string;
  hostname: string;
  status: 'idle' | 'running' | 'offline' | string;
  mode: string;
  poll_interval_ms: number;
  current_job_id: string;
  current_release_id: string;
  started_at: string;
  last_seen_at: string;
  updated_at: string;
  stale: boolean;
  stale_after?: string;
};

export type WorkerJobSnapshot = ReleaseJob;

export type WorkerSnapshot = {
  config: WorkerRuntimeConfig;
  stats: WorkerQueueStats;
  workers: WorkerHeartbeat[];
  jobs: WorkerJobSnapshot[];
};

export type User = {
  id: number;
  user_name: string;
  user_name_cn: string;
  user_email: string;
  user_type: number;
};

export type Privilege = {
  id: number;
  priv_name: string;
  description: string;
  priv_type: string;
  permission: string;
  is_builtin: boolean;
};

export type Role = {
  id: number;
  role_name: string;
  role_desc: string;
  is_builtin: boolean;
};

export type RoleMember = {
  id: number;
  role_id: number;
  member_email: string;
  on_duty: boolean;
};

export type UserAccess = {
  roles: string[];
  permissions: string[];
  global_roles: string[];
};

export type ProjectRole = {
  id: number;
  role_name: 'owner' | 'developer' | 'lead' | string;
  role_desc: string;
  is_builtin: boolean;
};
