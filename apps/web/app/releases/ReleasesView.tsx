"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { Plus, RefreshCw } from "lucide-react";
import { apiFetch } from "@/lib/api";
import { formatDateTime, formatFullDateTime, releaseStatusLabels } from "@/lib/presentation";
import type { Project, Release, WorkflowConfig } from "@/lib/types";
import { useProjectScope } from "@/components/ProjectScope";
import { EmptyState } from "@/components/EmptyState";
import { ReleaseStatusBadge } from "@/components/StatusBadge";
import { Alert } from "@/components/ui/alert";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export type ReleaseListQuery = {
  projectId: string;
};

export function ReleasesView({ projectId = "", adminView = false }: { projectId?: string; adminView?: boolean }) {
  const { selectedProjectId } = useProjectScope();
  const [items, setItems] = useState<Release[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [workflow, setWorkflow] = useState<WorkflowConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const query: ReleaseListQuery = { projectId: adminView ? "" : projectId || selectedProjectId };

  const load = async () => {
    if (!query.projectId && !adminView) return;
    setLoading(true); setError("");
    try {
      const [rows, projectRows] = await Promise.all([
        apiFetch<Release[]>(`/releases${query.projectId ? `?project_id=${query.projectId}` : ""}`),
        apiFetch<Project[]>("/projects")
      ]);
      const workflowConfig = query.projectId ? await apiFetch<WorkflowConfig>(`/projects/${query.projectId}/workflows/release`).catch(() => null) : null;
      setItems(rows); setProjects(projectRows); setWorkflow(workflowConfig);
    } catch (err: any) { setError(err.message || "发布工单加载失败"); }
    finally { setLoading(false); }
  };

  useEffect(() => { void load(); }, [query.projectId, adminView]);
  const statusLabel = (status: string) => workflow?.statuses.find((item) => item.id === status)?.name || releaseStatusLabels[status] || status;
  const project = projects.find((item) => item.id === query.projectId);
  const detailHref = (item: Release) => projectId ? `/projects/${projectId}/releases/${item.id}` : `/releases/${item.id}`;
  if (!query.projectId && !adminView) return <EmptyState title="先选择一个项目" description="选择项目后即可查看对应的发布工单。" action="管理项目" href="/projects" />;
  return (
    <div className="grid w-full min-w-0 grid-cols-1 gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4"><div><p className="text-sm font-medium text-primary">发布管理</p><h1 className="mt-1 text-2xl font-semibold">{adminView ? "全局发布工单" : project?.name || "当前项目"}</h1><p className="mt-1 text-sm text-muted-foreground">集中查看发布状态、审批与执行进度。</p></div><div className="flex gap-2"><Button variant="outline" onClick={load} disabled={loading}><RefreshCw className="h-4 w-4" />刷新</Button>{!adminView ? <Link href={projectId ? `/projects/${projectId}/releases/new` : "/releases/new"} className={buttonVariants({})}><Plus className="h-4 w-4" />新建发布工单</Link> : null}</div></div>
      {error ? <Alert>{error}</Alert> : null}
      <Card className="overflow-hidden"><CardContent className="p-0">{loading ? <ReleaseTableSkeleton /> : items.length === 0 ? <EmptyState title="还没有发布工单" description="当开发任务进入待发布阶段后，可以创建第一张发布工单。" action="新建发布工单" href={projectId ? `/projects/${projectId}/releases/new` : "/releases/new"} /> : <ReleaseListTable items={items} projects={projects} statusLabel={statusLabel} detailHref={detailHref} />}</CardContent></Card>
    </div>
  );
}

export function ReleaseListTable({ items, projects, statusLabel, detailHref }: { items: Release[]; projects: Project[]; statusLabel: (status: string) => string; detailHref: (release: Release) => string }) {
  return <Table><TableHeader className="bg-muted/60"><TableRow><TableHead>发布工单</TableHead><TableHead>所属项目</TableHead><TableHead>状态</TableHead><TableHead className="hidden lg:table-cell">创建时间</TableHead><TableHead className="hidden md:table-cell">最近更新</TableHead></TableRow></TableHeader><TableBody>{items.map((item) => <TableRow key={item.id}><TableCell><Link href={detailHref(item)} className="block font-medium text-primary hover:underline">{item.title}<div className="mt-1 text-xs font-normal text-muted-foreground md:hidden">更新于 {formatDateTime(item.updated_at)}</div></Link></TableCell><TableCell>{projects.find((project) => project.id === item.project_id)?.name || item.project_id}</TableCell><TableCell><ReleaseStatusBadge status={item.status} label={statusLabel(item.status)} /></TableCell><TableCell className="hidden whitespace-nowrap text-muted-foreground lg:table-cell">{formatFullDateTime(item.created_at)}</TableCell><TableCell className="hidden whitespace-nowrap text-muted-foreground md:table-cell">{formatDateTime(item.updated_at)}</TableCell></TableRow>)}</TableBody></Table>;
}

function ReleaseTableSkeleton() { return <div className="space-y-0">{[1,2,3,4,5].map((item) => <div key={item} className="flex items-center gap-6 border-b px-4 py-4"><Skeleton className="h-5 flex-1" /><Skeleton className="h-5 w-28" /><Skeleton className="h-6 w-20" /><Skeleton className="hidden h-5 w-28 lg:block" /><Skeleton className="hidden h-5 w-28 md:block" /></div>)}</div>; }
