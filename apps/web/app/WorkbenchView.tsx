"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { AlertTriangle, ArrowRight, CheckCircle2, ClipboardCheck, Rocket } from "lucide-react";
import { useProjectScope } from "@/components/ProjectScope";
import { EmptyState } from "@/components/EmptyState";
import { ReleaseStatusBadge, TaskStatusBadge } from "@/components/StatusBadge";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/presentation";
import type { Release, Task, WorkbenchResponse } from "@/lib/types";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function WorkbenchView() {
  const { selectedProjectId } = useProjectScope();
  const [data, setData] = useState<WorkbenchResponse | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!selectedProjectId) return;
    setData(null);
    setError("");
    apiFetch<WorkbenchResponse>(`/workbench?project_id=${selectedProjectId}`).then(setData).catch((err) => setError(err.message || "工作台加载失败"));
  }, [selectedProjectId]);

  if (!selectedProjectId) return <EmptyState title="先选择一个项目" description="工作台会根据当前项目整理你的开发任务和发布待办。" action="管理项目" href="/projects" />;
  if (error) return <Alert>{error}</Alert>;
  if (!data) return <WorkbenchSkeleton />;

  const hasWork = data.counts.my_open_tasks + data.counts.pending_approvals + data.counts.active_releases + data.counts.failed_releases > 0;
  return (
    <div className="grid w-full min-w-0 grid-cols-1 gap-6">
      <div>
        <p className="text-sm font-medium text-primary">我的工作台</p>
        <h1 className="mt-1 text-2xl font-semibold">今天需要处理什么？</h1>
        <p className="mt-1 text-sm text-muted-foreground">只展示与你有关、现在可以行动的任务和发布。</p>
      </div>
      {!hasWork ? <Card><EmptyState title="当前没有待办" description="你可以创建一个开发任务，或者前往发布管理查看历史工单。" action="创建开发任务" href="/tasks" icon={CheckCircle2} /></Card> : null}
      <div className="grid gap-5 lg:grid-cols-2">
        <WorkbenchSection title="我的开发任务" description={`${data.counts.my_open_tasks} 项未完成`} icon={ClipboardCheck} href="/tasks">
          {data.my_tasks.map((item) => <TaskItem key={item.id} item={item} />)}
        </WorkbenchSection>
        <WorkbenchSection title="等我审批" description={`${data.counts.pending_approvals} 张发布工单`} icon={CheckCircle2} href="/releases">
          {data.pending_approvals.map((item) => <ReleaseItem key={item.id} item={item} action="去审批" />)}
        </WorkbenchSection>
        <WorkbenchSection title="正在发布" description={`${data.counts.active_releases} 张进行中`} icon={Rocket} href="/releases">
          {data.active_releases.map((item) => <ReleaseItem key={item.id} item={item} action="查看进度" />)}
        </WorkbenchSection>
        <WorkbenchSection title="需要处理" description={`${data.counts.failed_releases} 张发布失败`} icon={AlertTriangle} href="/releases">
          {data.failed_releases.map((item) => <ReleaseItem key={item.id} item={item} action="查看原因" />)}
        </WorkbenchSection>
      </div>
    </div>
  );
}

function WorkbenchSection({ title, description, icon: Icon, href, children }: { title: string; description: string; icon: typeof Rocket; href: string; children: React.ReactNode }) {
  const empty = React.Children.count(children) === 0;
  return <Card className="overflow-hidden"><CardHeader className="flex-row items-start justify-between"><div><CardTitle className="flex items-center gap-2"><Icon className="h-4 w-4 text-primary" />{title}</CardTitle><CardDescription className="mt-1">{description}</CardDescription></div><Link href={href} className="text-sm font-medium text-primary">查看全部</Link></CardHeader><CardContent className="grid gap-1">{empty ? <div className="py-6 text-center text-sm text-muted-foreground">暂无需要处理的内容</div> : children}</CardContent></Card>;
}

function TaskItem({ item }: { item: Task }) {
  return <Link href={`/tasks/${item.id}`} className="flex items-center justify-between gap-3 rounded-lg px-3 py-3 hover:bg-muted/70"><div className="min-w-0"><div className="truncate text-sm font-medium">{item.title}</div><div className="mt-1 text-xs text-muted-foreground">更新于 {formatDateTime(item.updated_at)}</div></div><div className="flex shrink-0 items-center gap-2"><TaskStatusBadge status={item.status} /><ArrowRight className="h-4 w-4 text-muted-foreground" /></div></Link>;
}

function ReleaseItem({ item, action }: { item: Release; action: string }) {
  return <Link href={`/releases/${item.id}`} className="flex items-center justify-between gap-3 rounded-lg px-3 py-3 hover:bg-muted/70"><div className="min-w-0"><div className="truncate text-sm font-medium">{item.title}</div><div className="mt-1 text-xs text-muted-foreground">{action} · {formatDateTime(item.updated_at)}</div></div><div className="flex shrink-0 items-center gap-2"><ReleaseStatusBadge status={item.status} /><ArrowRight className="h-4 w-4 text-muted-foreground" /></div></Link>;
}

function WorkbenchSkeleton() {
  return <div className="grid w-full min-w-0 grid-cols-1 gap-6"><div className="space-y-2"><Skeleton className="h-4 w-20" /><Skeleton className="h-8 w-56" /><Skeleton className="h-4 w-80" /></div><div className="grid gap-5 lg:grid-cols-2">{[1,2,3,4].map((item) => <Card key={item}><CardHeader><Skeleton className="h-5 w-32" /><Skeleton className="h-4 w-24" /></CardHeader><CardContent className="space-y-3"><Skeleton className="h-12 w-full" /><Skeleton className="h-12 w-full" /></CardContent></Card>)}</div></div>;
}
