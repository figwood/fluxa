"use client";

import React, { FormEvent, useEffect, useMemo, useState } from "react";
import { RefreshCw } from "lucide-react";
import { apiFetch, postJSON } from "@/lib/api";
import type { GitlabRepository, Project, ProjectService, UserAccess } from "@/lib/types";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function ServicesView({ projectId = "" }: { projectId?: string }) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectRepos, setProjectRepos] = useState<GitlabRepository[]>([]);
  const [services, setServices] = useState<ProjectService[]>([]);
  const [repoForm, setRepoForm] = useState({ gitlab_project_id: "", name: "", path: "", url: "", default_branch: "main" });
  const [serviceForm, setServiceForm] = useState({
    project_id: projectId,
    gitlab_project_id: "",
    service_key: "",
    display_name: "",
    deploy_target: "portainer",
    mock_result: "success",
    image_name: "",
    module_path: "",
    deploy_note: ""
  });
  const [error, setError] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);

  const manageableProjects = useMemo(() => projects.filter((p) => p.current_user_role === "owner" || p.current_user_role === "lead"), [projects]);
  const projectOptions = useMemo(() => manageableProjects.map((p) => ({ label: `${p.name} (${p.key})`, value: p.id })), [manageableProjects]);
  const repoOptions = useMemo(() => projectRepos.map((r) => ({ label: `${r.name} #${r.gitlab_project_id}`, value: r.gitlab_project_id })), [projectRepos]);

  const load = async () => {
    setError("");
    try {
      const [projectRows, serviceRows, access] = await Promise.all([
        apiFetch<Project[]>("/projects"),
        apiFetch<ProjectService[]>(`/project-services${projectId ? `?project_id=${projectId}` : ""}`),
        apiFetch<UserAccess>("/auth/me")
      ]);
      setProjects(projectRows);
      setServices(serviceRows);
      setIsAdmin(access.global_roles.includes("admin"));
    } catch (err: any) {
      setError(err.message || "加载失败");
    }
  };

  useEffect(() => {
    load();
  }, []);

  useEffect(() => {
    if (!serviceForm.project_id) {
      setProjectRepos([]);
      return;
    }
    apiFetch<GitlabRepository[]>(`/projects/${serviceForm.project_id}/gitlab-repositories`)
      .then(setProjectRepos)
      .catch((err) => setError(err.message || "加载项目 GitLab 工程失败"));
  }, [serviceForm.project_id]);

  const createRepo = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    try {
      await postJSON<GitlabRepository>("/gitlab-repositories", {
        ...repoForm,
        gitlab_project_id: Number(repoForm.gitlab_project_id)
      });
      setRepoForm({ gitlab_project_id: "", name: "", path: "", url: "", default_branch: "main" });
      await load();
    } catch (err: any) {
      setError(err.message || "登记失败");
    }
  };

  const createService = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    try {
      await postJSON<ProjectService>("/project-services", {
        ...serviceForm,
        gitlab_project_id: Number(serviceForm.gitlab_project_id),
        deploy_config: {
          mock_result: serviceForm.mock_result,
          note: serviceForm.deploy_note
        }
      });
      setServiceForm({
        project_id: projectId,
        gitlab_project_id: "",
        service_key: "",
        display_name: "",
        deploy_target: "portainer",
        mock_result: "success",
        image_name: "",
        module_path: "",
        deploy_note: ""
      });
      await load();
    } catch (err: any) {
      setError(err.message || "绑定失败");
    }
  };

  return (
    <div className="grid gap-5">
      {!projectId ? <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">服务目录</h1>
          <p className="mt-1 text-sm text-muted-foreground">同一个 GitLab Project 可以绑定到多个项目内服务，发布时选择服务而不是裸仓库。</p>
        </div>
        <Button variant="outline" onClick={load}><RefreshCw className="h-4 w-4" />刷新</Button>
      </div> : null}
      {error ? <Alert>{error}</Alert> : null}
      <div className={projectId ? "grid gap-5" : "grid gap-5 xl:grid-cols-[0.9fr_1.1fr]"}>
        {isAdmin && !projectId ? <Card>
          <CardHeader>
            <CardTitle>登记 GitLab Project</CardTitle>
            <CardDescription>仓库资源可被多个业务项目复用。</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4" onSubmit={createRepo}>
              <Field label="GitLab Project ID">
                <Input type="number" min={1} value={repoForm.gitlab_project_id} onChange={(e) => setRepoForm({ ...repoForm, gitlab_project_id: e.target.value })} required />
              </Field>
              <Field label="名称">
                <Input value={repoForm.name} onChange={(e) => setRepoForm({ ...repoForm, name: e.target.value })} required />
              </Field>
              <Field label="路径">
                <Input value={repoForm.path} onChange={(e) => setRepoForm({ ...repoForm, path: e.target.value })} placeholder="group/project" />
              </Field>
              <Field label="URL">
                <Input value={repoForm.url} onChange={(e) => setRepoForm({ ...repoForm, url: e.target.value })} />
              </Field>
              <Field label="默认分支">
                <Input value={repoForm.default_branch} onChange={(e) => setRepoForm({ ...repoForm, default_branch: e.target.value })} />
              </Field>
              <Button type="submit">登记仓库</Button>
            </form>
          </CardContent>
        </Card> : null}
        {manageableProjects.length ? <Card>
          <CardHeader>
            <CardTitle>绑定项目内服务</CardTitle>
            <CardDescription>服务保存项目上下文、发布目标和 mock 配置。</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4" onSubmit={createService}>
              <div className="grid gap-4 md:grid-cols-2">
                {!projectId ? <Field label="业务项目">
                  <Select value={serviceForm.project_id} onChange={(e) => setServiceForm({ ...serviceForm, project_id: e.target.value })} options={projectOptions} placeholder="选择项目" required />
                </Field> : null}
                <Field label="GitLab Project">
                  <Select value={serviceForm.gitlab_project_id} onChange={(e) => setServiceForm({ ...serviceForm, gitlab_project_id: e.target.value })} options={repoOptions} placeholder="选择仓库" required />
                </Field>
                <Field label="服务 Key">
                  <Input value={serviceForm.service_key} onChange={(e) => setServiceForm({ ...serviceForm, service_key: e.target.value })} placeholder="checkout-api" required />
                </Field>
                <Field label="服务名称">
                  <Input value={serviceForm.display_name} onChange={(e) => setServiceForm({ ...serviceForm, display_name: e.target.value })} required />
                </Field>
                <Field label="发布方式">
                  <Select value={serviceForm.deploy_target} onChange={(e) => setServiceForm({ ...serviceForm, deploy_target: e.target.value })} options={[{ label: "Portainer", value: "portainer" }, { label: "Jenkins", value: "jenkins" }]} />
                </Field>
                <Field label="Mock 结果">
                  <Select value={serviceForm.mock_result} onChange={(e) => setServiceForm({ ...serviceForm, mock_result: e.target.value })} options={[{ label: "成功", value: "success" }, { label: "失败", value: "failed" }]} />
                </Field>
              </div>
              <Field label="镜像名">
                <Input value={serviceForm.image_name} onChange={(e) => setServiceForm({ ...serviceForm, image_name: e.target.value })} />
              </Field>
              <Field label="模块路径">
                <Input value={serviceForm.module_path} onChange={(e) => setServiceForm({ ...serviceForm, module_path: e.target.value })} placeholder="services/api" />
              </Field>
              <Field label="发布配置备注">
                <Input value={serviceForm.deploy_note} onChange={(e) => setServiceForm({ ...serviceForm, deploy_note: e.target.value })} />
              </Field>
              <Button type="submit">绑定服务</Button>
            </form>
          </CardContent>
        </Card> : null}
      </div>
      <Card>
        <CardHeader><CardTitle>项目内服务</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>服务 Key</TableHead>
                <TableHead>名称</TableHead>
                <TableHead>业务项目</TableHead>
                <TableHead>GitLab Project</TableHead>
                <TableHead>发布方式</TableHead>
                <TableHead>镜像</TableHead>
                <TableHead>模块路径</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {services.length === 0 ? <TableRow><TableCell colSpan={7}>暂无服务</TableCell></TableRow> : services.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-xs">{item.service_key}</TableCell>
                  <TableCell className="font-medium">{item.display_name}</TableCell>
                  <TableCell>{projects.find((p) => p.id === item.project_id)?.name || item.project_id}</TableCell>
                  <TableCell>{item.gitlab_project_id}</TableCell>
                  <TableCell><Badge variant="info">{item.deploy_target}</Badge></TableCell>
                  <TableCell>{item.image_name || "-"}</TableCell>
                  <TableCell>{item.module_path || "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
