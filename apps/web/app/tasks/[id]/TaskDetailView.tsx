"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, ArrowRight, CalendarDays, CheckCircle2, ChevronDown, Clock3, UserRound } from "lucide-react";
import { apiFetch, patchJSON } from "@/lib/api";
import { formatFullDateTime, taskStatusLabels } from "@/lib/presentation";
import type { Project, ProjectMemberList, Task, WorkflowConfig, WorkflowTransitionConfig } from "@/lib/types";
import { TaskStatusBadge } from "@/components/StatusBadge";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";

export function TaskDetailView({ taskId, projectId = "" }: { taskId: string; projectId?: string }) {
  const [task, setTask] = useState<Task | null>(null);
  const [project, setProject] = useState<Project | null>(null);
  const [workflow, setWorkflow] = useState<WorkflowConfig | null>(null);
  const [assigneeOptions, setAssigneeOptions] = useState<{ label: string; value: string }[]>([]);
  const [savingStatus, setSavingStatus] = useState(false);
  const [savingAssignee, setSavingAssignee] = useState(false);
  const [error, setError] = useState("");

  const load = async () => {
    setError("");
    const item = await apiFetch<Task>(`/tasks/${taskId}`);
    const [projects, workflowConfig, memberRows] = await Promise.all([
      apiFetch<Project[]>("/projects"),
      apiFetch<WorkflowConfig>(`/projects/${item.project_id}/workflows/task`).catch(() => null),
      apiFetch<ProjectMemberList>(`/projects/${item.project_id}/members`).catch(() => null)
    ]);
    setTask(item); setProject(projects.find((row) => row.id === item.project_id) || null); setWorkflow(workflowConfig);
    setAssigneeOptions(memberRows?.items.map((member) => ({ label: member.user_email ? `${member.user_name} (${member.user_email})` : member.user_name, value: member.user_name })) || []);
  };

  useEffect(() => { void load().catch((err: any) => setError(err.message || "加载任务失败")); }, [projectId, taskId]);
  const transitions = useMemo(() => workflow?.transitions.filter((item) => item.from_status === task?.status) || [], [workflow, task?.status]);
  const primary = transitions.find((item) => item.action !== "cancel");
  const secondary = transitions.filter((item) => item.id !== primary?.id);
  const statusLabel = (status: string) => workflow?.statuses.find((item) => item.id === status)?.name || taskStatusLabels[status] || status;

  const updateStatus = async (transition: WorkflowTransitionConfig) => {
    if (!task) return;
    setSavingStatus(true); setError("");
    try { setTask(await patchJSON<Task>(`/tasks/${task.id}/status`, { status: transition.to_status })); await load(); }
    catch (err: any) { setError(err.message || "任务推进失败"); }
    finally { setSavingStatus(false); }
  };
  const updateAssignee = async (assignee: string) => {
    if (!task || !assignee || assignee === task.assignee) return;
    setSavingAssignee(true); setError("");
    try { setTask(await patchJSON<Task>(`/tasks/${task.id}/assignee`, { assignee })); await load(); }
    catch (err: any) { setError(err.message || "负责人修改失败"); }
    finally { setSavingAssignee(false); }
  };

  if (error && !task) return <Alert>{error}</Alert>;
  if (!task) return <TaskDetailSkeleton />;
  const options = new Map(assigneeOptions.map((item) => [item.value, item.label]));
  if (task.assignee) options.set(task.assignee, options.get(task.assignee) || task.assignee);
  const schedule = task.details?.planned_start_at || task.details?.planned_end_at ? `${formatFullDateTime(task.details?.planned_start_at as string)} — ${formatFullDateTime(task.details?.planned_end_at as string)}` : "未设置工期";

  return (
    <div className="grid w-full min-w-0 grid-cols-1 gap-6">
      <div><Link href={projectId ? `/projects/${projectId}/tasks` : "/tasks"} className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft className="h-4 w-4" />返回开发任务</Link><div className="flex flex-wrap items-start justify-between gap-4"><div><div className="flex items-center gap-3"><h1 className="text-2xl font-semibold">{task.title}</h1><TaskStatusBadge status={task.status} label={statusLabel(task.status)} /></div><p className="mt-2 text-sm text-muted-foreground">{project?.name || task.project_id} · 创建于 {formatFullDateTime(task.created_at)}</p></div>{primary ? <Button disabled={savingStatus} onClick={() => updateStatus(primary)}>{primary.name}<ArrowRight className="h-4 w-4" /></Button> : <div className="flex items-center gap-2 text-sm text-muted-foreground"><CheckCircle2 className="h-4 w-4 text-emerald-600" />当前阶段已完成</div>}</div></div>
      {error ? <Alert>{error}</Alert> : null}
      <Card><CardHeader><CardTitle>任务进度</CardTitle><CardDescription>当前位于“{statusLabel(task.status)}”，使用右上角操作继续推进。</CardDescription></CardHeader><CardContent><div className="flex w-full min-w-0 items-center gap-2 pb-1">{(workflow?.stages || []).map((stage, index) => { const current = workflow?.statuses.find((status) => status.id === task.status)?.stage_id === stage.id; return <div key={stage.id} className="flex min-w-0 flex-1 items-center gap-2"><div className={`flex min-h-9 min-w-0 flex-1 items-center justify-center rounded-full px-2 py-1.5 text-center text-sm ${current ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground"}`}>{stage.name}</div>{index < (workflow?.stages.length || 0) - 1 ? <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground" /> : null}</div>; })}</div>{secondary.length ? <div className="mt-4 flex gap-2 border-t pt-4">{secondary.map((item) => <Button key={item.id} size="sm" variant={item.action === "cancel" ? "ghost" : "outline"} disabled={savingStatus} onClick={() => updateStatus(item)}>{item.name}</Button>)}</div> : null}</CardContent></Card>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_300px]"><Card><CardHeader><CardTitle>任务说明</CardTitle></CardHeader><CardContent><div className="whitespace-pre-wrap text-sm leading-6">{task.description || "暂无说明"}</div></CardContent></Card><Card><CardHeader><CardTitle>负责人</CardTitle></CardHeader><CardContent className="grid gap-4"><div className="flex items-center gap-2 text-sm"><UserRound className="h-4 w-4 text-muted-foreground" /><Select value={task.assignee} onChange={(event) => updateAssignee(event.target.value)} options={Array.from(options, ([value, label]) => ({ value, label }))} disabled={savingAssignee} /></div><div className="flex items-center gap-2 text-sm text-muted-foreground"><CalendarDays className="h-4 w-4" />{schedule}</div></CardContent></Card></div>
      <details className="group rounded-xl border bg-card"><summary className="flex cursor-pointer list-none items-center justify-between p-5"><div><div className="font-semibold">流转记录</div><div className="mt-1 text-sm text-muted-foreground">{task.events?.length || 0} 条操作记录</div></div><ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" /></summary><div className="grid gap-0 border-t px-5 py-2">{(task.events || []).map((event) => <div key={event.id} className="flex gap-3 border-b py-4 last:border-0"><div className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full bg-muted"><Clock3 className="h-3.5 w-3.5" /></div><div><div className="text-sm font-medium">{event.title}</div><div className="mt-1 text-sm text-muted-foreground">{event.actor || "系统"} · {formatFullDateTime(event.created_at)}</div>{event.message ? <div className="mt-1 text-sm text-muted-foreground">{event.message}</div> : null}</div></div>)}</div></details>
    </div>
  );
}

function TaskDetailSkeleton() { return <div className="grid w-full min-w-0 grid-cols-1 gap-6"><Skeleton className="h-9 w-72" /><Skeleton className="h-36 w-full" /><div className="grid gap-5 lg:grid-cols-2"><Skeleton className="h-48 w-full" /><Skeleton className="h-48 w-full" /></div></div>; }
