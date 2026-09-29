"use client";

import React, { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { CalendarDays, CheckSquare, ChevronDown, Plus, RefreshCw, Rocket } from "lucide-react";
import { apiFetch, postJSON } from "@/lib/api";
import type { Project, Task, WorkflowConfig } from "@/lib/types";
import { formatDateTime, formatFullDateTime, taskStatusLabels } from "@/lib/presentation";
import { Field } from "@/components/Field";
import { useProjectScope } from "@/components/ProjectScope";
import { EmptyState } from "@/components/EmptyState";
import { TaskStatusBadge } from "@/components/StatusBadge";
import { Alert } from "@/components/ui/alert";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogHeader } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

export function TasksView({ projectId = "", adminView = false }: { projectId?: string; adminView?: boolean }) {
  const { selectedProjectId } = useProjectScope();
  const [projects, setProjects] = useState<Project[]>([]);
  const [items, setItems] = useState<Task[]>([]);
  const [workflow, setWorkflow] = useState<WorkflowConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedTaskIDs, setSelectedTaskIDs] = useState<string[]>([]);
  const [form, setForm] = useState({ project_id: projectId, title: "", description: "", planned_start_at: "", planned_end_at: "" });
  const [createOpen, setCreateOpen] = useState(false);
  const [error, setError] = useState("");
  const effectiveProjectId = adminView ? "" : projectId || selectedProjectId;

  const load = async () => {
    if (!effectiveProjectId && !adminView) return;
    setLoading(true); setError("");
    try {
      const [projectRows, taskRows] = await Promise.all([
        apiFetch<Project[]>("/projects"),
        apiFetch<Task[]>(`/tasks${effectiveProjectId ? `?project_id=${effectiveProjectId}` : ""}`)
      ]);
      const workflowConfig = effectiveProjectId ? await apiFetch<WorkflowConfig>(`/projects/${effectiveProjectId}/workflows/task`) : null;
      setProjects(projectRows); setItems(taskRows); setWorkflow(workflowConfig);
    } catch (err: any) { setError(err.message || "加载失败"); }
    finally { setLoading(false); }
  };

  useEffect(() => {
    setSelectedTaskIDs([]);
    setForm((current) => ({ ...current, project_id: effectiveProjectId }));
    void load();
  }, [effectiveProjectId, adminView]);

  const statusLabel = (status: string) => workflow?.statuses.find((item) => item.id === status)?.name || taskStatusLabels[status] || status;
  const create = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setError("");
    try {
      await postJSON<Task>("/tasks", { project_id: form.project_id, title: form.title, description: form.description, details: { planned_start_at: form.planned_start_at || undefined, planned_end_at: form.planned_end_at || undefined } });
      setForm({ project_id: effectiveProjectId, title: "", description: "", planned_start_at: "", planned_end_at: "" });
      setCreateOpen(false); await load();
    } catch (err: any) { setError(err.message || "创建失败"); }
  };

  const toggle = (id: string) => setSelectedTaskIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  const projectName = projects.find((item) => item.id === effectiveProjectId)?.name;
  const detailHref = (task: Task) => projectId ? `/projects/${projectId}/tasks/${task.id}` : `/tasks/${task.id}`;
  const releaseHref = `${projectId ? `/projects/${projectId}` : ""}/releases/new?task_ids=${selectedTaskIDs.join(",")}`;

  if (!effectiveProjectId && !adminView) return <EmptyState title="先选择一个项目" description="选择项目后即可查看和创建开发任务。" action="管理项目" href="/projects" />;
  return (
    <div className="grid w-full min-w-0 grid-cols-1 gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4"><div><p className="text-sm font-medium text-primary">开发任务</p><h1 className="mt-1 text-2xl font-semibold">{adminView ? "全局任务" : projectName || "当前项目"}</h1><p className="mt-1 text-sm text-muted-foreground">集中查看任务状态、负责人和推进进度。</p></div><div className="flex gap-2"><Button variant="outline" onClick={load} disabled={loading}><RefreshCw className="h-4 w-4" />刷新</Button>{!adminView ? <Button onClick={() => setCreateOpen(true)}><Plus className="h-4 w-4" />创建任务</Button> : null}</div></div>
      {error ? <Alert>{error}</Alert> : null}
      <Card className="overflow-hidden"><CardContent className="p-0">{loading ? <TaskTableSkeleton /> : items.length === 0 ? <EmptyState title="还没有开发任务" description="创建第一项任务，系统会引导你逐步推进到发布。" action="创建任务" onAction={() => setCreateOpen(true)} icon={CheckSquare} /> : <TaskListTable items={items} projects={projects} statusLabel={statusLabel} detailHref={detailHref} selected={selectedTaskIDs} onToggle={toggle} />}</CardContent></Card>
      {selectedTaskIDs.length ? <div className="sticky bottom-4 z-20 mx-auto flex items-center gap-4 rounded-xl border bg-card px-4 py-3 shadow-lg"><span className="text-sm">已选择 {selectedTaskIDs.length} 项待发布任务</span><Link href={releaseHref} className={buttonVariants({ size: "sm" })}><Rocket className="h-4 w-4" />创建发布工单</Link></div> : null}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}><DialogHeader title="创建开发任务" description="先记录目标，其他信息可以稍后补充。" onClose={() => setCreateOpen(false)} /><form className="grid gap-4 p-5" onSubmit={create}>{!projectId && adminView ? <Field label="项目"><Select value={form.project_id} onChange={(event) => setForm({ ...form, project_id: event.target.value })} options={projects.map((item) => ({ label: item.name, value: item.id }))} required /></Field> : null}<Field label="任务标题"><Input autoFocus value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} placeholder="例如：完成订单导出功能" required /></Field><Field label="任务说明"><Textarea value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} placeholder="说明目标、范围或验收要求" /></Field><details className="group rounded-lg border p-3"><summary className="flex cursor-pointer list-none items-center gap-2 text-sm font-medium"><CalendarDays className="h-4 w-4 text-muted-foreground" />更多信息<ChevronDown className="ml-auto h-4 w-4 transition-transform group-open:rotate-180" /></summary><div className="mt-4 grid gap-4 sm:grid-cols-2"><Field label="计划开始"><Input type="datetime-local" value={form.planned_start_at} onChange={(event) => setForm({ ...form, planned_start_at: event.target.value })} /></Field><Field label="计划结束"><Input type="datetime-local" value={form.planned_end_at} onChange={(event) => setForm({ ...form, planned_end_at: event.target.value })} /></Field></div></details><div className="flex justify-end gap-2"><Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>取消</Button><Button type="submit">创建任务</Button></div></form></Dialog>
    </div>
  );
}

function TaskListTable({ items, projects, statusLabel, detailHref, selected, onToggle }: { items: Task[]; projects: Project[]; statusLabel: (status: string) => string; detailHref: (task: Task) => string; selected: string[]; onToggle: (id: string) => void }) {
  return <Table><TableHeader className="bg-muted/60"><TableRow><TableHead className="w-10"><span className="sr-only">选择</span></TableHead><TableHead>开发任务</TableHead><TableHead>所属项目</TableHead><TableHead>状态</TableHead><TableHead className="hidden lg:table-cell">负责人</TableHead><TableHead className="hidden xl:table-cell">创建时间</TableHead><TableHead className="hidden md:table-cell">最近更新</TableHead></TableRow></TableHeader><TableBody>{items.map((item) => {
    const selectable = item.status === "publishing";
    return <TableRow key={item.id}><TableCell>{selectable ? <input className="h-4 w-4 accent-primary" aria-label={`选择 ${item.title}`} type="checkbox" checked={selected.includes(item.id)} onChange={() => onToggle(item.id)} /> : null}</TableCell><TableCell><Link href={detailHref(item)} className="block font-medium text-primary hover:underline"><span>{item.title}</span><div className="mt-1 text-xs font-normal text-muted-foreground md:hidden">更新于 {formatDateTime(item.updated_at)}</div></Link></TableCell><TableCell>{projects.find((project) => project.id === item.project_id)?.name || item.project_id}</TableCell><TableCell><TaskStatusBadge status={item.status} label={statusLabel(item.status)} /></TableCell><TableCell className="hidden lg:table-cell">{item.assignee || "待分配"}</TableCell><TableCell className="hidden whitespace-nowrap text-muted-foreground xl:table-cell">{formatFullDateTime(item.created_at)}</TableCell><TableCell className="hidden whitespace-nowrap text-muted-foreground md:table-cell">{formatDateTime(item.updated_at)}</TableCell></TableRow>;
  })}</TableBody></Table>;
}

function TaskTableSkeleton() { return <div className="space-y-0">{[1,2,3,4,5].map((item) => <div key={item} className="flex items-center gap-6 border-b px-4 py-4"><Skeleton className="h-4 w-4" /><Skeleton className="h-5 flex-1" /><Skeleton className="h-5 w-28" /><Skeleton className="h-6 w-20" /><Skeleton className="hidden h-5 w-24 lg:block" /><Skeleton className="hidden h-5 w-28 xl:block" /><Skeleton className="hidden h-5 w-28 md:block" /></div>)}</div>; }
