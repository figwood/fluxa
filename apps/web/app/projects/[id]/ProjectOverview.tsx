"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { GitlabRepository, Project, ProjectMemberList, ProjectService, Release, Task } from "@/lib/types";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function ProjectOverview({ projectId }: { projectId: string }) {
  const [project, setProject] = useState<Project | null>(null);
  const [stats, setStats] = useState({ members: 0, repositories: 0, services: 0, tasks: 0, releases: 0 });
  const [tasks, setTasks] = useState<Task[]>([]);
  const [releases, setReleases] = useState<Release[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      apiFetch<Project>(`/projects/${projectId}`),
      apiFetch<ProjectMemberList>(`/projects/${projectId}/members`),
      apiFetch<GitlabRepository[]>(`/projects/${projectId}/gitlab-repositories`),
      apiFetch<ProjectService[]>(`/project-services?project_id=${projectId}`),
      apiFetch<Task[]>(`/tasks?project_id=${projectId}`),
      apiFetch<Release[]>(`/releases?project_id=${projectId}`)
    ]).then(([p, m, r, s, t, rel]) => {
      setProject(p); setTasks(t); setReleases(rel);
      setStats({ members: m.items.length, repositories: r.length, services: s.length, tasks: t.length, releases: rel.length });
    }).catch((err) => setError(err.message || "加载项目概览失败"));
  }, [projectId]);

  const cards = [["任务单", stats.tasks, "tasks"], ["发布单", stats.releases, "releases"], ["可发布服务", stats.services, "gitlab"], ["GitLab 工程", stats.repositories, "gitlab"], ["项目成员", stats.members, "members"]] as const;
  return <div className="grid gap-5">
    <div>
      <Link href="/projects" className="mb-2 inline-flex h-8 items-center justify-center gap-2 rounded-md px-2.5 text-xs font-medium hover:bg-accent hover:text-accent-foreground"><ArrowLeft className="h-4 w-4" />返回项目列表</Link>
      <h1 className="text-2xl font-semibold">{project?.name || "项目概览"}</h1>
      <p className="mt-1 text-sm text-muted-foreground">{project ? `${project.key} · ${project.description || "暂无描述"}` : "加载中..."}</p>
    </div>
    {error ? <Alert>{error}</Alert> : null}
    <div className="grid gap-3 md:grid-cols-5">{cards.map(([label, count, route], index) => <Link key={label} href={`/projects/${projectId}/${route}`}><Card className="h-full transition-colors hover:bg-secondary/40"><CardContent className={index < 2 ? "pt-5" : "p-4"}><div className="text-sm text-muted-foreground">{label}</div><div className={index < 2 ? "mt-2 text-3xl font-semibold" : "mt-2 text-2xl font-semibold"}>{count}</div></CardContent></Card></Link>)}</div>
    <div className="grid gap-5 lg:grid-cols-2">
      <Card><CardHeader className="py-4"><CardTitle>近期任务</CardTitle></CardHeader><CardContent className="grid gap-3">{tasks.slice(0, 5).map((item) => <Link key={item.id} href={`/projects/${projectId}/tasks`} className="flex items-center justify-between border-b pb-2 text-sm hover:text-primary"><span>{item.title}</span><Badge>{item.status}</Badge></Link>)}{tasks.length === 0 ? <span className="text-sm text-muted-foreground">暂无任务</span> : null}</CardContent></Card>
      <Card><CardHeader className="py-4"><CardTitle>近期发布</CardTitle></CardHeader><CardContent className="grid gap-3">{releases.slice(0, 5).map((item) => <Link key={item.id} href={`/projects/${projectId}/releases/${item.id}`} className="flex items-center justify-between border-b pb-2 text-sm hover:text-primary"><span>{item.title}</span><Badge>{item.status}</Badge></Link>)}{releases.length === 0 ? <span className="text-sm text-muted-foreground">暂无发布单</span> : null}</CardContent></Card>
    </div>
    <Card className="bg-card/70 shadow-none"><CardHeader className="py-4"><CardTitle>项目配置</CardTitle></CardHeader><CardContent className="grid gap-3 md:grid-cols-4"><Link className="rounded-md border p-3 text-sm hover:bg-secondary" href={`/projects/${projectId}/members`}>添加项目成员<br/><span className="text-muted-foreground">当前 {stats.members} 人</span></Link><Link className="rounded-md border p-3 text-sm hover:bg-secondary" href={`/projects/${projectId}/gitlab`}>关联 GitLab 工程与服务<br/><span className="text-muted-foreground">当前 {stats.repositories} 个工程</span></Link><Link className="rounded-md border p-3 text-sm hover:bg-secondary" href={`/projects/${projectId}/workflows`}>确认工作流模板<br/><span className="text-muted-foreground">标准模板已自动创建</span></Link><Link className="rounded-md border p-3 text-sm hover:bg-secondary" href={`/projects/${projectId}/gitlab`}>配置 CI Token<br/><span className="text-muted-foreground">制品上报和 DEV 自动部署</span></Link></CardContent></Card>
  </div>;
}
