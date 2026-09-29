"use client";

import React, { FormEvent, useEffect, useMemo, useState } from "react";
import { Pencil, Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { apiFetch, deleteJSON, postJSON, putJSON } from "@/lib/api";
import type { Privilege, ProjectRole, Role, RoleMember } from "@/lib/types";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

type RoleForm = {
  role_name: string;
  role_desc: string;
};

const emptyForm: RoleForm = { role_name: "", role_desc: "" };

export default function RolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [privileges, setPrivileges] = useState<Privilege[]>([]);
  const [selectedRoleID, setSelectedRoleID] = useState<number | null>(null);
  const [rolePrivileges, setRolePrivileges] = useState<number[]>([]);
  const [memberText, setMemberText] = useState("");
  const [projectRoles, setProjectRoles] = useState<ProjectRole[]>([]);
  const [form, setForm] = useState<RoleForm>(emptyForm);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const selectedRole = useMemo(() => roles.find((item) => item.id === selectedRoleID) || null, [roles, selectedRoleID]);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const [nextRoles, nextPrivileges, nextProjectRoles] = await Promise.all([
        apiFetch<Role[]>("/role/list"),
        apiFetch<Privilege[]>("/privilege/list"),
        apiFetch<ProjectRole[]>("/project-roles")
      ]);
      setRoles(nextRoles);
      setPrivileges(nextPrivileges);
      setProjectRoles(nextProjectRoles);
      if (!selectedRoleID && nextRoles.length > 0) {
        await selectRole(nextRoles[0].id);
      }
    } catch (err: any) {
      setError(err.message || "加载失败");
    } finally {
      setLoading(false);
    }
  };

  const selectRole = async (id: number) => {
    setSelectedRoleID(id);
    setError("");
    try {
      const [members, assigned] = await Promise.all([
        apiFetch<RoleMember[]>(`/role/${id}/member/list`),
        apiFetch<Privilege[]>(`/role/${id}/privileges`)
      ]);
      setMemberText(members.map((item) => item.member_email).join("\n"));
      setRolePrivileges(assigned.map((item) => item.id));
    } catch (err: any) {
      setError(err.message || "加载角色详情失败");
    }
  };

  useEffect(() => {
    load();
  }, []);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    try {
      if (editingID) {
        await putJSON<Role>(`/role/${editingID}`, form);
      } else {
        await postJSON<Role>("/role", form);
      }
      setForm(emptyForm);
      setEditingID(null);
      await load();
    } catch (err: any) {
      setError(err.message || "保存失败");
    }
  };

  const edit = (item: Role) => {
    setEditingID(item.id);
    setForm({ role_name: item.role_name, role_desc: item.role_desc });
  };

  const remove = async (id: number) => {
    setError("");
    try {
      await deleteJSON<void>(`/role/${id}`);
      if (selectedRoleID === id) {
        setSelectedRoleID(null);
        setMemberText("");
        setRolePrivileges([]);
      }
      await load();
    } catch (err: any) {
      setError(err.message || "删除失败");
    }
  };

  const togglePrivilege = (id: number) => {
    setRolePrivileges((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  };

  const saveBindings = async () => {
    if (!selectedRoleID) return;
    setError("");
    try {
      await putJSON<Privilege[]>(`/role/${selectedRoleID}/privileges`, { privilege_ids: rolePrivileges });
      const emails = memberText.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean);
      await putJSON<RoleMember[]>(`/role/${selectedRoleID}/members`, { emails });
      await selectRole(selectedRoleID);
    } catch (err: any) {
      setError(err.message || "保存绑定失败");
    }
  };

  return (
    <div className="grid gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">角色管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">角色聚合成员和权限，用户最终权限由角色权限去重得到。</p>
        </div>
        <Button variant="outline" onClick={load}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle>{editingID ? "编辑角色" : "创建角色"}</CardTitle>
          <CardDescription>创建后在角色列表中选择角色，再维护成员和权限。</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-4 md:grid-cols-[1fr_2fr_auto]" onSubmit={submit}>
            <Field label="角色名称">
              <Input value={form.role_name} onChange={(e) => setForm({ ...form, role_name: e.target.value })} required />
            </Field>
            <Field label="描述">
              <Input value={form.role_desc} onChange={(e) => setForm({ ...form, role_desc: e.target.value })} />
            </Field>
            <div className="flex items-end gap-2">
              <Button type="submit">
                <Plus className="h-4 w-4" />
                {editingID ? "保存" : "创建"}
              </Button>
              {editingID ? (
                <Button type="button" variant="outline" onClick={() => { setEditingID(null); setForm(emptyForm); }}>
                  取消
                </Button>
              ) : null}
            </div>
          </form>
        </CardContent>
      </Card>

      <div className="grid gap-5 lg:grid-cols-[1.15fr_0.85fr]">
        <Card>
          <CardHeader>
            <CardTitle>角色列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>角色</TableHead>
                  <TableHead>描述</TableHead>
                  <TableHead className="w-48">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow><TableCell colSpan={3}>加载中...</TableCell></TableRow>
                ) : roles.length === 0 ? (
                  <TableRow><TableCell colSpan={3}>暂无角色</TableCell></TableRow>
                ) : roles.map((item) => (
                  <TableRow key={item.id} className={selectedRoleID === item.id ? "bg-primary/10" : undefined}>
                    <TableCell>
                      <button className="text-left font-medium" onClick={() => selectRole(item.id)}>{item.role_name}</button>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{item.role_desc || "-"}</TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        <Button size="sm" variant="outline" onClick={() => edit(item)}><Pencil className="h-3.5 w-3.5" />编辑</Button>
                        <Button size="sm" variant="destructive" onClick={() => remove(item.id)} disabled={item.is_builtin}><Trash2 className="h-3.5 w-3.5" />删除</Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{selectedRole ? `${selectedRole.role_name} 绑定` : "角色绑定"}</CardTitle>
            <CardDescription>成员邮箱每行一个，也可以用逗号分隔。</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <Field label="成员邮箱">
              <Textarea rows={6} value={memberText} onChange={(e) => setMemberText(e.target.value)} disabled={!selectedRoleID} />
            </Field>
            <div className="grid gap-2">
              <div className="text-sm font-medium">权限</div>
              <div className="grid max-h-64 gap-2 overflow-auto rounded-md border bg-background p-3">
                {privileges.length === 0 ? (
                  <div className="text-sm text-muted-foreground">暂无权限</div>
                ) : privileges.map((item) => (
                  <label key={item.id} className="flex items-start gap-2 text-sm">
                    <input
                      type="checkbox"
                      className="mt-1"
                      checked={rolePrivileges.includes(item.id)}
                      onChange={() => togglePrivilege(item.id)}
                      disabled={!selectedRoleID}
                    />
                    <span>
                      <span className="font-medium">{item.priv_name}</span>
                      <span className="ml-2 font-mono text-xs text-muted-foreground">{item.permission}</span>
                    </span>
                  </label>
                ))}
              </div>
            </div>
            <Button onClick={saveBindings} disabled={!selectedRoleID}>
              <Save className="h-4 w-4" />
              保存绑定
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>项目角色</CardTitle>
          <CardDescription>项目权限使用固定的 owner、lead、developer 三级角色；成员关系在项目页面维护。</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>角色</TableHead>
                <TableHead>描述</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {projectRoles.length === 0 ? (
                <TableRow><TableCell colSpan={2}>暂无项目角色</TableCell></TableRow>
              ) : projectRoles.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.role_name}</TableCell>
                  <TableCell className="text-muted-foreground">{item.role_desc || "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
