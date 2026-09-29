"use client";

import React, { FormEvent, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, ExternalLink, Plus, Search, Trash2 } from "lucide-react";
import { apiFetch, deleteJSON, postJSON, putJSON } from "@/lib/api";
import type { GitlabRepository, Project, ProjectMember, ProjectMemberCandidate, ProjectMemberList, RemoteGitlabProject } from "@/lib/types";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const roleOptions = [
  { label: "Owner", value: "owner" },
  { label: "Lead", value: "lead" },
  { label: "Developer", value: "developer" }
];

export default function ProjectDetail({ projectId, section }: { projectId: string; section?: "members" | "repositories" }) {
  const [project, setProject] = useState<Project | null>(null);
  const [activeTab, setActiveTab] = useState<"members" | "repositories">(section || "members");
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [candidates, setCandidates] = useState<ProjectMemberCandidate[]>([]);
  const [repositories, setRepositories] = useState<GitlabRepository[]>([]);
  const [candidateID, setCandidateID] = useState("");
  const [memberRole, setMemberRole] = useState("developer");
  const [keyword, setKeyword] = useState("");
  const [searchResults, setSearchResults] = useState<RemoteGitlabProject[]>([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState("");

  const canManageMembers = project?.current_user_role === "owner";
  const canManageRepositories = project?.current_user_role === "owner" || project?.current_user_role === "lead";

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [projectRow, memberRows, repositoryRows] = await Promise.all([
        apiFetch<Project>(`/projects/${projectId}`),
        apiFetch<ProjectMemberList>(`/projects/${projectId}/members`),
        apiFetch<GitlabRepository[]>(`/projects/${projectId}/gitlab-repositories`)
      ]);
      setProject(projectRow);
      setMembers(memberRows.items);
      setRepositories(repositoryRows);
      if (memberRows.can_manage) {
        setCandidates(await apiFetch<ProjectMemberCandidate[]>(`/projects/${projectId}/member-candidates`));
      } else {
        setCandidates([]);
      }
    } catch (err: any) {
      setError(err.message || "加载项目详情失败");
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => { void load(); }, [load]);

  const addMember = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!candidateID) return;
    try {
      await postJSON(`/projects/${projectId}/members`, { user_id: Number(candidateID), role_name: memberRole });
      setCandidateID("");
      await load();
    } catch (err: any) { setError(err.message || "添加成员失败"); }
  };

  const updateMember = async (member: ProjectMember, roleName: string) => {
    try {
      await putJSON(`/projects/${projectId}/members/${member.user_id}`, { role_name: roleName });
      await load();
    } catch (err: any) { setError(err.message || "更新成员角色失败"); }
  };

  const removeMember = async (member: ProjectMember) => {
    if (!window.confirm(`确定移除成员 ${member.user_name}？`)) return;
    try {
      await deleteJSON(`/projects/${projectId}/members/${member.user_id}`);
      await load();
    } catch (err: any) { setError(err.message || "移除成员失败"); }
  };

  const searchProjects = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!keyword.trim()) return;
    setSearching(true);
    setError("");
    try {
      setSearchResults(await apiFetch<RemoteGitlabProject[]>(`/gitlab/projects?keyword=${encodeURIComponent(keyword.trim())}`));
    } catch (err: any) { setError(err.message || "搜索 GitLab 工程失败"); }
    finally { setSearching(false); }
  };

  const attachRepository = async (item: RemoteGitlabProject) => {
    try {
      await postJSON(`/projects/${projectId}/gitlab-repositories`, { gitlab_project_id: item.id });
      setSearchResults((rows) => rows.filter((row) => row.id !== item.id));
      await load();
    } catch (err: any) { setError(err.message || "关联 GitLab 工程失败"); }
  };

  const detachRepository = async (item: GitlabRepository) => {
    if (!window.confirm(`确定解除与 ${item.name} 的关联？此操作不会删除 GitLab 仓库。`)) return;
    try {
      await deleteJSON(`/projects/${projectId}/gitlab-repositories/${item.gitlab_project_id}`);
      await load();
    } catch (err: any) { setError(err.message || "工程仍被项目服务使用，无法解除关联"); }
  };

  const attachedIDs = new Set(repositories.map((item) => item.gitlab_project_id));

  return (
    <div className="grid gap-5">
      {!section ? <div className="flex items-start justify-between gap-4">
        <div>
          <Link href="/projects" className="mb-2 inline-flex h-8 items-center justify-center gap-2 rounded-md px-2.5 text-xs font-medium hover:bg-accent hover:text-accent-foreground"><ArrowLeft className="h-4 w-4" />返回项目列表</Link>
          <h1 className="text-2xl font-semibold">{project?.name || "项目详情"}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{project ? `${project.key} · ${project.description || "暂无描述"}` : "加载中..."}</p>
        </div>
      </div> : null}
      {error ? <Alert>{error}</Alert> : null}
      {!section ? <div className="flex gap-2 border-b pb-3">
        <Button variant={activeTab === "members" ? "default" : "ghost"} onClick={() => setActiveTab("members")}>项目成员</Button>
        <Button variant={activeTab === "repositories" ? "default" : "ghost"} onClick={() => setActiveTab("repositories")}>GitLab 工程</Button>
      </div> : null}

      {activeTab === "members" ? <Card>
        <CardHeader><CardTitle>项目成员</CardTitle><CardDescription>Owner 管理成员；Lead 管理服务与发布；Developer 管理任务并提交发布单。</CardDescription></CardHeader>
        <CardContent className="grid gap-4">
          {canManageMembers ? <form className="grid gap-3 md:grid-cols-[2fr_1fr_auto]" onSubmit={addMember}>
            <Select value={candidateID} onChange={(e) => setCandidateID(e.target.value)} options={candidates.map((item) => ({ label: `${item.user_name} (${item.user_email})`, value: item.user_id }))} placeholder={candidates.length ? "选择用户" : "没有可添加用户"} required />
            <Select value={memberRole} onChange={(e) => setMemberRole(e.target.value)} options={roleOptions} />
            <Button type="submit" disabled={!candidateID}><Plus className="h-4 w-4" />添加成员</Button>
          </form> : null}
          <Table><TableHeader><TableRow><TableHead>用户</TableHead><TableHead>邮箱</TableHead><TableHead className="w-48">项目角色</TableHead>{canManageMembers ? <TableHead className="w-24">操作</TableHead> : null}</TableRow></TableHeader>
            <TableBody>{loading ? <TableRow><TableCell colSpan={4}>加载中...</TableCell></TableRow> : members.length === 0 ? <TableRow><TableCell colSpan={4}>暂无成员</TableCell></TableRow> : members.map((member) => <TableRow key={member.id}>
              <TableCell className="font-medium">{member.user_name}</TableCell><TableCell className="text-muted-foreground">{member.user_email}</TableCell>
              <TableCell>{canManageMembers ? <Select value={member.role_name} onChange={(e) => updateMember(member, e.target.value)} options={roleOptions} /> : <Badge>{member.role_name}</Badge>}</TableCell>
              {canManageMembers ? <TableCell><Button size="sm" variant="destructive" onClick={() => removeMember(member)} aria-label={`移除成员 ${member.user_name}`}><Trash2 className="h-3.5 w-3.5" /></Button></TableCell> : null}
            </TableRow>)}</TableBody>
          </Table>
        </CardContent>
      </Card> : <div className="grid gap-5">
        {canManageRepositories ? <Card><CardHeader><CardTitle>关联 GitLab 工程</CardTitle><CardDescription>搜索 GitLab 中已有工程并关联到当前业务项目。</CardDescription></CardHeader><CardContent className="grid gap-4">
          <form className="flex gap-2" onSubmit={searchProjects}><Input value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="输入工程名称或路径" /><Button type="submit" disabled={searching || !keyword.trim()}><Search className="h-4 w-4" />{searching ? "搜索中" : "搜索"}</Button></form>
          {searchResults.filter((item) => !attachedIDs.has(item.id)).length ? <Table><TableHeader><TableRow><TableHead>工程</TableHead><TableHead>路径</TableHead><TableHead className="w-24">操作</TableHead></TableRow></TableHeader><TableBody>{searchResults.filter((item) => !attachedIDs.has(item.id)).map((item) => <TableRow key={item.id}><TableCell>{item.name} <span className="font-mono text-xs text-muted-foreground">#{item.id}</span></TableCell><TableCell>{item.path_with_namespace}</TableCell><TableCell><Button size="sm" onClick={() => attachRepository(item)}><Plus className="h-3.5 w-3.5" />关联</Button></TableCell></TableRow>)}</TableBody></Table> : null}
        </CardContent></Card> : null}
        <Card><CardHeader><CardTitle>已关联工程</CardTitle><CardDescription>解除关联不会删除真实 GitLab 仓库；被项目服务使用的工程不可解除。</CardDescription></CardHeader><CardContent>
          <Table><TableHeader><TableRow><TableHead>工程</TableHead><TableHead>路径</TableHead><TableHead>默认分支</TableHead><TableHead className="w-28">操作</TableHead></TableRow></TableHeader><TableBody>{loading ? <TableRow><TableCell colSpan={4}>加载中...</TableCell></TableRow> : repositories.length === 0 ? <TableRow><TableCell colSpan={4}>暂无关联工程</TableCell></TableRow> : repositories.map((item) => <TableRow key={item.id}><TableCell className="font-medium">{item.url ? <a href={item.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 hover:underline">{item.name}<ExternalLink className="h-3.5 w-3.5" /></a> : item.name} <span className="font-mono text-xs text-muted-foreground">#{item.gitlab_project_id}</span></TableCell><TableCell>{item.path || "-"}</TableCell><TableCell><Badge>{item.default_branch}</Badge></TableCell><TableCell>{canManageRepositories ? <Button size="sm" variant="destructive" onClick={() => detachRepository(item)}><Trash2 className="h-3.5 w-3.5" />移除</Button> : "-"}</TableCell></TableRow>)}</TableBody></Table>
        </CardContent></Card>
      </div>}
    </div>
  );
}
