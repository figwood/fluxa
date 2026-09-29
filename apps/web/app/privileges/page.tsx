"use client";

import React, { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { apiFetch, deleteJSON, postJSON, putJSON } from "@/lib/api";
import type { Privilege } from "@/lib/types";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

type PrivilegeForm = {
  priv_name: string;
  description: string;
  priv_type: string;
  permission: string;
};

const emptyForm: PrivilegeForm = { priv_name: "", description: "", priv_type: "application", permission: "" };

export default function PrivilegesPage() {
  const [items, setItems] = useState<Privilege[]>([]);
  const [form, setForm] = useState<PrivilegeForm>(emptyForm);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await apiFetch<Privilege[]>("/privilege/list"));
    } catch (err: any) {
      setError(err.message || "加载失败");
    } finally {
      setLoading(false);
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
        await putJSON<Privilege>(`/privilege/${editingID}`, {
          description: form.description,
          priv_type: form.priv_type,
          permission: form.permission
        });
      } else {
        await postJSON<Privilege>("/privilege", form);
      }
      setEditingID(null);
      setForm(emptyForm);
      await load();
    } catch (err: any) {
      setError(err.message || "保存失败");
    }
  };

  const edit = (item: Privilege) => {
    setEditingID(item.id);
    setForm({
      priv_name: item.priv_name,
      description: item.description,
      priv_type: item.priv_type,
      permission: item.permission
    });
  };

  const remove = async (id: number) => {
    setError("");
    try {
      await deleteJSON<void>(`/privilege/${id}`);
      await load();
    } catch (err: any) {
      setError(err.message || "删除失败");
    }
  };

  return (
    <div className="grid gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">权限管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">权限定义绑定到角色后，形成用户最终权限集合。</p>
        </div>
        <Button variant="outline" onClick={load}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle>{editingID ? "编辑权限" : "创建权限"}</CardTitle>
          <CardDescription>权限标识建议使用资源:动作，例如 release:create。</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-4 md:grid-cols-[1fr_1.3fr_160px_1fr_auto]" onSubmit={submit}>
            <Field label="权限名称">
              <Input value={form.priv_name} disabled={!!editingID} onChange={(e) => setForm({ ...form, priv_name: e.target.value })} required />
            </Field>
            <Field label="描述">
              <Input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} required />
            </Field>
            <Field label="类型">
              <Select
                value={form.priv_type}
                onChange={(e) => setForm({ ...form, priv_type: e.target.value })}
                options={[
                  { label: "应用", value: "application" },
                  { label: "通配", value: "wildcard" }
                ]}
              />
            </Field>
            <Field label="权限标识">
              <Input value={form.permission} onChange={(e) => setForm({ ...form, permission: e.target.value })} required />
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

      <Card>
        <CardHeader>
          <CardTitle>权限列表</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                <TableHead>描述</TableHead>
                <TableHead>类型</TableHead>
                <TableHead>权限标识</TableHead>
                <TableHead className="w-40">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow><TableCell colSpan={5}>加载中...</TableCell></TableRow>
              ) : items.length === 0 ? (
                <TableRow><TableCell colSpan={5}>暂无权限</TableCell></TableRow>
              ) : items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.priv_name}</TableCell>
                  <TableCell>{item.description}</TableCell>
                  <TableCell>{item.priv_type}</TableCell>
                  <TableCell className="font-mono text-xs">{item.permission}</TableCell>
                  <TableCell>
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => edit(item)} disabled={item.is_builtin}><Pencil className="h-3.5 w-3.5" />编辑</Button>
                      <Button size="sm" variant="destructive" onClick={() => remove(item.id)} disabled={item.is_builtin}><Trash2 className="h-3.5 w-3.5" />删除</Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
