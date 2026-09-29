"use client";

import React, { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, ArrowRight, Check, PackageCheck, Rocket, Wrench } from "lucide-react";
import { apiFetch, postJSON } from "@/lib/api";
import type { Artifact, Project, ProjectService, Release, Task } from "@/lib/types";
import { useProjectScope } from "@/components/ProjectScope";
import { EmptyState } from "@/components/EmptyState";
import { TaskStatusBadge } from "@/components/StatusBadge";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

const steps = ["选择任务", "选择服务与制品", "确认发布"];

export function NewReleaseView({ projectId = "" }: { projectId?: string }) {
  const router = useRouter();
  const { selectedProjectId } = useProjectScope();
  const effectiveProjectId = projectId || selectedProjectId;
  const [step, setStep] = useState(1);
  const [projects, setProjects] = useState<Project[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [services, setServices] = useState<ProjectService[]>([]);
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [selectedTaskIDs, setSelectedTaskIDs] = useState<string[]>([]);
  const [selectedServiceIDs, setSelectedServiceIDs] = useState<string[]>([]);
  const [selectedArtifacts, setSelectedArtifacts] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ title: "", description: "" });

  useEffect(() => {
    if (!effectiveProjectId) { setLoading(false); return; }
    setLoading(true); setError("");
    Promise.all([
      apiFetch<Project[]>("/projects"),
      apiFetch<Task[]>(`/tasks?status=publishing&project_id=${effectiveProjectId}`),
      apiFetch<ProjectService[]>(`/project-services?project_id=${effectiveProjectId}`),
      apiFetch<Artifact[]>(`/projects/${effectiveProjectId}/artifacts`)
    ]).then(([projectRows, taskRows, serviceRows, artifactRows]) => {
      setProjects(projectRows); setTasks(taskRows); setServices(serviceRows);
      setArtifacts(artifactRows.filter((item) => item.ref_type === "tag" || item.ref === "main"));
      const ids = new URLSearchParams(window.location.search).get("task_ids")?.split(",").filter((id) => taskRows.some((task) => task.id === id)) || [];
      setSelectedTaskIDs(ids);
    }).catch((err) => setError(err.message || "发布准备信息加载失败")).finally(() => setLoading(false));
  }, [effectiveProjectId]);

  const project = projects.find((item) => item.id === effectiveProjectId);
  const selectedTasks = tasks.filter((item) => selectedTaskIDs.includes(item.id));
  const selectedServices = services.filter((item) => selectedServiceIDs.includes(item.id));
  const canContinue = step === 1 ? selectedTaskIDs.length > 0 : step === 2 ? selectedServiceIDs.length > 0 && selectedServiceIDs.every((id) => selectedArtifacts[id]) : form.title.trim().length > 0;
  const toggleTask = (id: string) => setSelectedTaskIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  const toggleService = (id: string) => setSelectedServiceIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canContinue || !effectiveProjectId) return;
    setSaving(true); setError("");
    try {
      const release = await postJSON<Release>("/releases", {
        project_id: effectiveProjectId, title: form.title, description: form.description, details: {}, task_ids: selectedTaskIDs,
        services: selectedServiceIDs.map((id, index) => ({ project_service_id: id, artifact_id: selectedArtifacts[id], order_index: index + 1 }))
      });
      router.push(projectId ? `/projects/${projectId}/releases/${release.id}` : `/releases/${release.id}`);
    } catch (err: any) { setError(err.message || "创建发布工单失败"); setSaving(false); }
  };

  if (!effectiveProjectId) return <EmptyState title="先选择一个项目" description="发布工单必须属于一个明确的项目。" action="返回发布管理" href="/releases" />;
  return (
    <form className="grid w-full min-w-0 grid-cols-1 gap-6" onSubmit={submit}>
      <div><Link href={projectId ? `/projects/${projectId}/releases` : "/releases"} className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft className="h-4 w-4" />返回发布管理</Link><p className="text-sm font-medium text-primary">新建发布工单</p><h1 className="mt-1 text-2xl font-semibold">发布 {project?.name || "当前项目"}</h1><p className="mt-1 text-sm text-muted-foreground">按步骤选择任务和构建制品，确认后再创建工单。</p></div>
      <div className="grid grid-cols-3 gap-2">{steps.map((label, index) => { const number = index + 1; const active = number === step; const complete = number < step; return <div key={label} className={`flex items-center gap-2 rounded-lg border px-3 py-3 text-sm ${active ? "border-primary bg-primary/5 font-medium text-primary" : "text-muted-foreground"}`}><div className={`grid h-6 w-6 place-items-center rounded-full text-xs ${active || complete ? "bg-primary text-primary-foreground" : "bg-muted"}`}>{complete ? <Check className="h-3.5 w-3.5" /> : number}</div><span className="hidden sm:inline">{label}</span></div>; })}</div>
      {error ? <Alert>{error}</Alert> : null}
      {loading ? <WizardSkeleton /> : step === 1 ? <Card><CardHeader><CardTitle>选择待发布任务</CardTitle><CardDescription>这里只显示已完成开发、进入“待发布”的任务。</CardDescription></CardHeader><CardContent className="p-0">{tasks.length === 0 ? <EmptyState title="没有可发布的任务" description="先到开发任务中完成工作，并将任务推进到待发布阶段。" action="前往开发任务" href="/tasks" icon={PackageCheck} /> : <Table><TableHeader><TableRow><TableHead className="w-12" /><TableHead>任务</TableHead><TableHead>负责人</TableHead><TableHead>状态</TableHead></TableRow></TableHeader><TableBody>{tasks.map((task) => <TableRow key={task.id}><TableCell><input className="h-4 w-4 accent-primary" type="checkbox" aria-label={`选择 ${task.title}`} checked={selectedTaskIDs.includes(task.id)} onChange={() => toggleTask(task.id)} /></TableCell><TableCell className="font-medium">{task.title}</TableCell><TableCell>{task.assignee || "待分配"}</TableCell><TableCell><TaskStatusBadge status={task.status} /></TableCell></TableRow>)}</TableBody></Table>}</CardContent></Card> : null}
      {!loading && step === 2 ? <Card><CardHeader><CardTitle>选择服务与构建制品</CardTitle><CardDescription>每个服务必须选择一份由 CI 上报的 main 或 tag 制品。</CardDescription></CardHeader><CardContent className="p-0">{services.length === 0 ? <EmptyState title="还没有可发布服务" description="请先在项目设置中关联 GitLab 工程并配置发布服务。" action="配置发布服务" href={`/projects/${effectiveProjectId}/gitlab`} icon={Wrench} /> : <Table><TableHeader><TableRow><TableHead className="w-12" /><TableHead>服务</TableHead><TableHead>发布方式</TableHead><TableHead>构建制品</TableHead></TableRow></TableHeader><TableBody>{services.map((service) => { const options = artifacts.filter((item) => item.project_service_id === service.id).map((item) => ({ label: `${item.ref} · ${item.image_tag || item.image_digest.slice(0, 18)}`, value: item.id })); return <TableRow key={service.id}><TableCell><input className="h-4 w-4 accent-primary" type="checkbox" aria-label={`选择 ${service.display_name}`} checked={selectedServiceIDs.includes(service.id)} onChange={() => toggleService(service.id)} /></TableCell><TableCell><div className="font-medium">{service.display_name}</div><div className="mt-1 text-xs text-muted-foreground">{service.service_key}</div></TableCell><TableCell>{service.deploy_target}</TableCell><TableCell className="min-w-64">{options.length ? <Select value={selectedArtifacts[service.id] || ""} onChange={(event) => { const artifactId = event.target.value; setSelectedArtifacts((current) => ({ ...current, [service.id]: artifactId })); if (artifactId && !selectedServiceIDs.includes(service.id)) setSelectedServiceIDs((current) => [...current, service.id]); }} options={options} placeholder="选择 CI 制品" /> : <div className="text-sm text-amber-700">暂无 main 或 tag 制品</div>}</TableCell></TableRow>; })}</TableBody></Table>}</CardContent></Card> : null}
      {!loading && step === 3 ? <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_320px]"><Card><CardHeader><CardTitle>发布工单信息</CardTitle><CardDescription>名称用于团队在列表中快速识别这次发布。</CardDescription></CardHeader><CardContent className="grid gap-4"><Field label="发布名称"><Input autoFocus value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} placeholder="例如：订单服务 7 月版本发布" required /></Field><Field label="发布说明"><Textarea value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} placeholder="可选：说明变更范围或注意事项" /></Field></CardContent></Card><Card><CardHeader><CardTitle>发布范围</CardTitle></CardHeader><CardContent className="grid gap-4 text-sm"><div><div className="text-muted-foreground">关联任务</div><div className="mt-1 font-medium">{selectedTasks.map((item) => item.title).join("、")}</div></div><div><div className="text-muted-foreground">发布服务</div><div className="mt-1 font-medium">{selectedServices.map((item) => item.display_name).join("、")}</div></div><div className="rounded-lg bg-muted p-3 text-muted-foreground"><Rocket className="mb-2 h-4 w-4 text-primary" />创建后将按照项目发布流程进入审批与执行。</div></CardContent></Card></div> : null}
      {!loading ? <div className="flex justify-between"><Button type="button" variant="outline" onClick={() => step === 1 ? router.push("/releases") : setStep((value) => value - 1)}>{step === 1 ? "取消" : "上一步"}</Button>{step < 3 ? <Button type="button" disabled={!canContinue} onClick={() => setStep((value) => value + 1)}>下一步<ArrowRight className="h-4 w-4" /></Button> : <Button type="submit" disabled={!canContinue || saving}><Rocket className="h-4 w-4" />{saving ? "正在创建..." : "创建发布工单"}</Button>}</div> : null}
    </form>
  );
}

function WizardSkeleton() { return <Card><CardHeader><Skeleton className="h-6 w-40" /><Skeleton className="h-4 w-64" /></CardHeader><CardContent className="space-y-4"><Skeleton className="h-14 w-full" /><Skeleton className="h-14 w-full" /><Skeleton className="h-14 w-full" /></CardContent></Card>; }
