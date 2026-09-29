"use client";

import React, { useEffect, useState } from "react";
import { apiFetch, postJSON, putJSON } from "@/lib/api";
import type { Artifact, Deployment, EnvironmentPolicy, Project } from "@/lib/types";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export default function DeliveryPanel({ projectId }: { projectId: string }) {
  const [artifacts,setArtifacts]=useState<Artifact[]>([]);const [deployments,setDeployments]=useState<Deployment[]>([]);const [policies,setPolicies]=useState<EnvironmentPolicy[]>([]);const [project,setProject]=useState<Project|null>(null);const [token,setToken]=useState("");const [error,setError]=useState("");
  const load=async()=>{try{const [a,d,p,projectRow]=await Promise.all([apiFetch<Artifact[]>(`/projects/${projectId}/artifacts`),apiFetch<Deployment[]>(`/projects/${projectId}/deployments`),apiFetch<EnvironmentPolicy[]>(`/projects/${projectId}/environment-policies`),apiFetch<Project>(`/projects/${projectId}`)]);setArtifacts(a);setDeployments(d);setPolicies(p);setProject(projectRow);}catch(err:any){setError(err.message||"加载交付数据失败")}};
  useEffect(()=>{void load()},[projectId]);
  const canManage=project?.current_user_role==="owner";
  return <div className="grid gap-5">
    {error?<Alert>{error}</Alert>:null}
    <Card><CardHeader><CardTitle>CI 制品上报</CardTitle><CardDescription>CI 使用项目 Token 调用 POST /api/v1/ci/artifacts；Token 仅在轮换后展示一次。</CardDescription></CardHeader><CardContent className="grid gap-3">{canManage?<Button className="w-fit" variant="outline" onClick={async()=>{try{const result=await postJSON<{token:string}>(`/projects/${projectId}/ci-token/rotate`,{});setToken(result.token)}catch(err:any){setError(err.message)}}}>生成/轮换 CI Token</Button>:null}{token?<pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs">Authorization: Bearer {token}</pre>:null}</CardContent></Card>
    <Card><CardHeader><CardTitle>环境门禁</CardTitle></CardHeader><CardContent><Table><TableHeader><TableRow><TableHead>环境</TableHead><TableHead>自动部署</TableHead><TableHead>需要审批</TableHead><TableHead>完成任务</TableHead></TableRow></TableHeader><TableBody>{policies.map((item)=><TableRow key={item.environment}><TableCell className="font-semibold uppercase">{item.environment}</TableCell><TableCell>{item.auto_deploy?"是":"否"}</TableCell><TableCell>{item.approval_required?"是":"否"}</TableCell><TableCell>{item.completes_tasks?"是":"否"}</TableCell></TableRow>)}</TableBody></Table>{canManage?<Button className="mt-4" variant="outline" onClick={()=>putJSON(`/projects/${projectId}/environment-policies`,policies).then(load).catch((err)=>setError(err.message))}>保存默认门禁</Button>:null}</CardContent></Card>
    <Card><CardHeader><CardTitle>构建制品</CardTitle></CardHeader><CardContent><Table><TableHeader><TableRow><TableHead>Ref</TableHead><TableHead>镜像</TableHead><TableHead>Digest</TableHead><TableHead>Pipeline</TableHead></TableRow></TableHeader><TableBody>{artifacts.length===0?<TableRow><TableCell colSpan={4}>暂无 CI 上报制品</TableCell></TableRow>:artifacts.slice(0,20).map((item)=><TableRow key={item.id}><TableCell><Badge variant="outline">{item.ref_type}</Badge> {item.ref}</TableCell><TableCell>{item.image_repository}:{item.image_tag}</TableCell><TableCell className="max-w-64 truncate font-mono text-xs">{item.image_digest}</TableCell><TableCell>{item.pipeline_url?<a className="text-primary hover:underline" href={item.pipeline_url} target="_blank" rel="noreferrer">#{item.pipeline_id}</a>:item.pipeline_id||"-"}</TableCell></TableRow>)}</TableBody></Table></CardContent></Card>
    <Card><CardHeader><CardTitle>部署记录</CardTitle><CardDescription>DEV 自动部署与 STG/PROD 正式发布统一在此审计。</CardDescription></CardHeader><CardContent><Table><TableHeader><TableRow><TableHead>环境</TableHead><TableHead>来源</TableHead><TableHead>状态</TableHead><TableHead>信息</TableHead></TableRow></TableHeader><TableBody>{deployments.length===0?<TableRow><TableCell colSpan={4}>暂无部署记录</TableCell></TableRow>:deployments.slice(0,20).map((item)=><TableRow key={item.id}><TableCell className="uppercase">{item.environment}</TableCell><TableCell>{item.source}</TableCell><TableCell><Badge>{item.status}</Badge></TableCell><TableCell>{item.message||"-"}</TableCell></TableRow>)}</TableBody></Table></CardContent></Card>
  </div>;
}
