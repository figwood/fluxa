"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, ArrowRight, CheckCircle2, ChevronDown, Clock3, ExternalLink, RefreshCw, RotateCcw, Server, XCircle } from "lucide-react";
import { apiFetch, patchJSON, postJSON } from "@/lib/api";
import { formatFullDateTime, releaseStatusLabels } from "@/lib/presentation";
import type { Project, Release, ReleaseService, ReleaseStatusUpdate, WorkflowConfig, WorkflowExecution, WorkflowTransitionConfig } from "@/lib/types";
import { ReleaseStatusBadge, TaskStatusBadge } from "@/components/StatusBadge";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogHeader } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";

type PendingAction = { kind: "cancel" } | { kind: "transition"; transition: WorkflowTransitionConfig } | { kind: "retry" };

export function ReleaseDetailView({ projectId = "", releaseId = "" }: { projectId?: string; releaseId?: string }) {
  const params = useParams<{ id: string }>();
  const id = releaseId || params.id;
  const [release, setRelease] = useState<Release | null>(null);
  const [project, setProject] = useState<Project | null>(null);
  const [workflow, setWorkflow] = useState<WorkflowConfig | null>(null);
  const [execution, setExecution] = useState<WorkflowExecution | null>(null);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);

  const load = async () => {
    const [item, projects] = await Promise.all([apiFetch<Release>(`/releases/${id}`), apiFetch<Project[]>("/projects")]);
    const workflowConfig = await apiFetch<WorkflowConfig>(`/projects/${item.project_id}/workflows/release`);
    const run = await apiFetch<WorkflowExecution>(`/releases/${id}/execution`).catch(() => null);
    setRelease(item); setProject(projects.find((candidate) => candidate.id === item.project_id) || null); setWorkflow(workflowConfig); setExecution(run);
  };

  useEffect(() => { void load().catch((err) => setError(err.message || "发布工单加载失败")); }, [id]);
  useEffect(() => {
    if (!release || !["queued", "running", "publishing"].includes(release.status)) return;
    const timer = window.setInterval(() => { void load().catch(() => undefined); }, 2500);
    return () => window.clearInterval(timer);
  }, [release?.status, id]);

  const role = project?.current_user_role || "developer";
  const statusLabel = (status: string) => workflow?.statuses.find((item) => item.id === status)?.name || releaseStatusLabels[status] || status;
  const transitions = useMemo(() => {
    if (!release || !workflow) return [];
    if (execution) return [];
    return workflow.transitions.filter((item) => item.from_status === release.status && !["success", "failed"].includes(item.to_status) && (!["approved", "publishing"].includes(item.to_status) || ["owner", "lead"].includes(role)));
  }, [release, workflow, role, execution]);
  const primary = transitions.find((item) => item.action !== "cancel");
  const secondary = transitions.filter((item) => item.id !== primary?.id);
  const canRetry = execution?.phase === "waiting_retry" && ["owner", "lead"].includes(role);
  const canCancel = execution && !["completed", "cancelled"].includes(execution.phase) && ["owner", "lead"].includes(role);

  const execute = async () => {
    if (!pendingAction) return;
    setSaving(true); setError("");
    try {
      if (pendingAction.kind === "retry") setExecution(await postJSON<WorkflowExecution>(`/releases/${id}/retry`, {}));
      else if (pendingAction.kind === "cancel") { if (!execution) return; setExecution(await postJSON<WorkflowExecution>(`/executions/${execution.id}/cancel`, {})); }
      else {
        const transition = pendingAction.transition;
        if (release?.status === "draft" && transition.action === "submit") setRelease(await postJSON<Release>(`/releases/${id}/submit`, {}));
        else if (release?.status === "pending_approval" && transition.action === "approve") setRelease(await postJSON<Release>(`/releases/${id}/approve`, {}));
        else if (["queued", "publishing"].includes(transition.to_status)) {
          setExecution(await postJSON<WorkflowExecution>(`/releases/${id}/publish`, {}));
        } else {
          const result = await patchJSON<ReleaseStatusUpdate>(`/releases/${id}/status`, { status: transition.to_status });
          if (result.execution) setExecution(result.execution); setRelease(result.release);
        }
      }
      setPendingAction(null); await load();
    } catch (err: any) { setError(err.message || "操作失败"); }
    finally { setSaving(false); }
  };

  if (error && !release) return <Alert>{error}</Alert>;
  if (!release) return <ReleaseDetailSkeleton />;
  const actionTitle = pendingAction?.kind === "retry" ? "确认重试发布？" : pendingAction?.kind === "cancel" ? "确认取消发布？" : pendingAction?.transition.name || "确认操作";
  const actionDescription = pendingAction?.kind === "retry" ? "系统会从失败步骤恢复，跳过已成功的服务；结果未确定的部署将使用原操作标识查询或重试。" : pendingAction?.kind === "cancel" ? "停止后续步骤。已经提交给外部系统的部署可能需要核对结果，取消不会自动回滚。" : `发布工单将从“${statusLabel(release.status)}”推进到“${statusLabel(pendingAction?.kind === "transition" ? pendingAction.transition.to_status : release.status)}”。`;
  const phaseLabels: Record<string, string> = { running: "正在执行", waiting_approval: "等待审批", waiting_time: "等待发布时间", waiting_retry: "等待重试", completed: "执行完成", failed: "执行失败", cancelled: "已取消" };
  const decide = async (taskId: string, decision: "approved" | "rejected") => { setSaving(true); setError(""); try { setExecution(await postJSON<WorkflowExecution>(`/executions/${execution?.id}/approvals/${taskId}`, { decision })); await load(); } catch (err: any) { setError(err.message || "审批失败"); } finally { setSaving(false); } };


  return (
    <div className="grid w-full min-w-0 grid-cols-1 gap-6">
      <div><Link href={projectId ? `/projects/${projectId}/releases` : "/releases"} className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft className="h-4 w-4" />返回发布管理</Link><div className="flex flex-wrap items-start justify-between gap-4"><div><div className="flex flex-wrap items-center gap-3"><h1 className="text-2xl font-semibold">{release.title}</h1><ReleaseStatusBadge status={release.status} label={statusLabel(release.status)} /></div><p className="mt-2 text-sm text-muted-foreground">{project?.name || release.project_id} · 创建于 {formatFullDateTime(release.created_at)}</p></div><div className="flex gap-2"><Button variant="outline" onClick={() => void load()}><RefreshCw className="h-4 w-4" />刷新</Button>{canCancel ? <Button variant="outline" onClick={() => setPendingAction({ kind: "cancel" })}>取消发布</Button> : null}{canRetry ? <Button onClick={() => setPendingAction({ kind: "retry" })}><RotateCcw className="h-4 w-4" />重试发布</Button> : primary ? <Button onClick={() => setPendingAction({ kind: "transition", transition: primary })}>{primary.name}<ArrowRight className="h-4 w-4" /></Button> : null}</div></div></div>
      {error ? <Alert>{error}</Alert> : null}
      {execution ? <Card><CardHeader><CardTitle>内部执行阶段</CardTitle><CardDescription>发布单主状态保持“{statusLabel(release.status)}”，审批、等待和重试在内部步骤中推进。</CardDescription></CardHeader><CardContent className="grid gap-4"><div className="flex flex-wrap items-center gap-3 text-sm"><ReleaseStatusBadge status={execution.phase} label={phaseLabels[execution.phase] || execution.phase} /><span className="font-mono text-muted-foreground">{execution.current_step_id}</span>{execution.phase === "waiting_time" && execution.wake_up_at ? <span>恢复时间：{formatFullDateTime(execution.wake_up_at)}</span> : null}</div>{execution.last_error ? <Alert>{execution.last_error}</Alert> : null}{canRetry ? <Button className="w-fit" disabled={saving} onClick={() => setPendingAction({ kind: "retry" })}><RotateCcw className="h-4 w-4" />重试当前步骤</Button> : null}<div className="grid gap-2">{(execution.approvals || []).filter((task) => execution.phase === "waiting_approval" && task.step_id === execution.current_step_id && task.decision === "pending").map((task) => <div key={task.id} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3 text-sm"><span>{task.approver} · {task.step_id}</span><div className="flex gap-2"><Button size="sm" variant="outline" disabled={saving} onClick={() => void decide(task.id, "rejected")}>拒绝</Button><Button size="sm" disabled={saving} onClick={() => void decide(task.id, "approved")}>批准</Button></div></div>)}</div></CardContent></Card> : null}
      <Card><CardHeader><CardTitle>发布进度</CardTitle><CardDescription>{["queued", "running", "publishing"].includes(release.status) ? "系统正在自动执行，页面会持续更新。" : `当前处于“${statusLabel(release.status)}”。`}</CardDescription></CardHeader><CardContent><ReleaseProgress release={release} workflow={workflow} />{secondary.length ? <div className="mt-5 flex gap-2 border-t pt-4">{secondary.map((item) => <Button key={item.id} size="sm" variant={item.action === "cancel" ? "ghost" : "outline"} onClick={() => setPendingAction({ kind: "transition", transition: item })}>{item.name}</Button>)}</div> : null}</CardContent></Card>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_320px]"><Card><CardHeader><CardTitle>发布说明</CardTitle></CardHeader><CardContent><div className="whitespace-pre-wrap text-sm leading-6">{release.description || "暂无说明"}</div></CardContent></Card><Card><CardHeader><CardTitle>发布范围</CardTitle></CardHeader><CardContent className="grid gap-4 text-sm"><div><div className="text-muted-foreground">关联任务</div><div className="mt-1 font-medium">{release.tasks?.length || 0} 项</div></div><div><div className="text-muted-foreground">发布服务</div><div className="mt-1 font-medium">{release.services?.length || 0} 项</div></div></CardContent></Card></div>
      <Card><CardHeader><CardTitle>服务发布情况</CardTitle><CardDescription>按顺序经过 STG 和 PROD 环境执行。</CardDescription></CardHeader><CardContent className="grid gap-3">{(release.services || []).map((service) => <ServiceProgress key={service.id} service={service} />)}</CardContent></Card>
      <Card><CardHeader><CardTitle>关联开发任务</CardTitle></CardHeader><CardContent className="divide-y">{(release.tasks || []).map((task) => <div key={task.id} className="flex items-center justify-between py-3 first:pt-0 last:pb-0"><Link href={`/tasks/${task.task_id}`} className="font-medium hover:text-primary">{task.title}</Link><TaskStatusBadge status={task.status} /></div>)}</CardContent></Card>
      <details className="group rounded-xl border bg-card"><summary className="flex cursor-pointer list-none items-center justify-between p-5"><div><div className="font-semibold">技术详情与完整记录</div><div className="mt-1 text-sm text-muted-foreground">服务标识、制品、外部系统信息及 {release.events?.length || 0} 条事件</div></div><ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" /></summary><div className="grid gap-6 border-t p-5"><div className="grid gap-3">{(release.services || []).map((service) => <div key={service.id} className="rounded-lg bg-muted p-3 text-xs text-muted-foreground"><div className="font-mono text-foreground">{service.service_key}</div><div className="mt-1">Artifact: {service.artifact_id || "-"}</div><div>External ID: {service.external_id || "-"}</div>{service.message ? <div>Message: {service.message}</div> : null}</div>)}</div><div className="grid gap-0">{(release.events || []).map((event) => <div key={event.id} className="flex gap-3 border-b py-4 last:border-0"><Clock3 className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><div className="text-sm font-medium">{event.title}</div><div className="mt-1 text-sm text-muted-foreground">{event.actor || "系统"} · {formatFullDateTime(event.created_at)}</div>{event.message ? <div className="mt-1 text-sm text-muted-foreground">{event.message}</div> : null}</div></div>)}</div></div></details>
      <Dialog open={pendingAction !== null} onOpenChange={(open) => !open && setPendingAction(null)}><DialogHeader title={actionTitle} description={actionDescription} onClose={() => setPendingAction(null)} /><div className="flex justify-end gap-2 p-5"><Button variant="outline" onClick={() => setPendingAction(null)}>取消</Button><Button disabled={saving} onClick={execute}>{saving ? "处理中..." : "确认"}</Button></div></Dialog>
    </div>
  );
}

function ReleaseProgress({ release, workflow }: { release: Release; workflow: WorkflowConfig | null }) {
  const currentStage = workflow?.statuses.find((item) => item.id === release.status)?.stage_id;
  const currentIndex = Math.max(0, workflow?.stages.findIndex((item) => item.id === currentStage) || 0);
  return <div className="flex w-full min-w-0 items-center gap-2 pb-1">{(workflow?.stages || []).map((stage, index) => <div key={stage.id} className="flex min-w-0 flex-1 items-center gap-2"><div className={`flex min-h-9 min-w-0 flex-1 items-center justify-center gap-2 rounded-full px-2 py-1.5 text-center text-sm ${index === currentIndex ? "bg-primary text-primary-foreground" : index < currentIndex ? "bg-emerald-50 text-emerald-700" : "bg-muted text-muted-foreground"}`}>{index < currentIndex ? <CheckCircle2 className="h-3.5 w-3.5 shrink-0" /> : null}{stage.name}</div>{index < (workflow?.stages.length || 0) - 1 ? <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground" /> : null}</div>)}</div>;
}

function ServiceProgress({ service }: { service: ReleaseService }) {
  const success = service.status === "success"; const failed = service.status === "failed";
  const label = { pending: "等待执行", running: "正在发布", success: "发布成功", failed: "发布失败", skipped: "已跳过" }[service.status] || service.status;
  return <div className="flex flex-wrap items-center gap-3 rounded-lg border p-4"><div className={`grid h-9 w-9 place-items-center rounded-full ${success ? "bg-emerald-50 text-emerald-700" : failed ? "bg-rose-50 text-rose-700" : "bg-muted text-muted-foreground"}`}>{success ? <CheckCircle2 className="h-5 w-5" /> : failed ? <XCircle className="h-5 w-5" /> : <Server className="h-5 w-5" />}</div><div className="min-w-48 flex-1"><div className="font-medium">{service.display_name}</div><div className="mt-1 text-sm text-muted-foreground">{service.message || label}</div></div><ReleaseStatusBadge status={service.status} label={label} />{service.external_url ? <a className="inline-flex items-center gap-1 text-sm text-primary" href={service.external_url} target="_blank" rel="noreferrer">外部详情<ExternalLink className="h-3.5 w-3.5" /></a> : null}</div>;
}

function ReleaseDetailSkeleton() { return <div className="grid w-full min-w-0 grid-cols-1 gap-6"><Skeleton className="h-9 w-80" /><Skeleton className="h-40 w-full" /><div className="grid gap-5 lg:grid-cols-2"><Skeleton className="h-44 w-full" /><Skeleton className="h-44 w-full" /></div></div>; }
