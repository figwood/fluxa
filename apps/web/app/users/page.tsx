"use client";

import React, { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus, RefreshCw, RotateCcw, Trash2 } from "lucide-react";
import { apiFetch, deleteJSON, postJSON, putJSON } from "@/lib/api";
import type { User } from "@/lib/types";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

type UserForm = {
  user_name: string;
  user_name_cn: string;
  user_email: string;
  user_type: string;
  initial_password: string;
};

const emptyForm: UserForm = { user_name: "", user_name_cn: "", user_email: "", user_type: "0", initial_password: "" };

export default function UsersPage() {
  const [items, setItems] = useState<User[]>([]);
  const [form, setForm] = useState<UserForm>(emptyForm);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await apiFetch<User[]>("/user/list"));
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
    const body = { ...form, user_type: Number(form.user_type || 0) };
    try {
      if (editingID) {
        const { initial_password: _, ...updates } = body;
        await putJSON<User>(`/user/${editingID}`, updates);
      } else {
        await postJSON<User>("/user", body);
      }
      setForm(emptyForm);
      setEditingID(null);
      await load();
    } catch (err: any) {
      setError(err.message || "保存失败");
    }
  };

  const edit = (item: User) => {
    setEditingID(item.id);
    setForm({
      user_name: item.user_name,
      user_name_cn: item.user_name_cn,
      user_email: item.user_email,
      user_type: String(item.user_type),
      initial_password: ""
    });
  };

  const remove = async (id: number) => {
    setError("");
    try {
      await deleteJSON<void>(`/user/${id}`);
      await load();
    } catch (err: any) {
      setError(err.message || "删除失败");
    }
  };

  const resetPassword = async (id: number) => {
    setError("");
    const password = window.prompt("输入不少于 12 位的新密码");
    if (!password) return;
    try {
      await putJSON<void>(`/user/${id}/password`, { password });
    } catch (err: any) {
      setError(err.message || "重置失败");
    }
  };

  return (
    <div className="grid gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">用户管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">维护平台用户、角色和登录凭据。</p>
        </div>
        <Button variant="outline" onClick={load}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle>{editingID ? "编辑用户" : "创建用户"}</CardTitle>
          <CardDescription>用户名、中文名和邮箱用于角色成员匹配。</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-4 md:grid-cols-2 xl:grid-cols-[1fr_1fr_1.4fr_120px_1.2fr_auto]" onSubmit={submit}>
            <Field label="用户名">
              <Input value={form.user_name} onChange={(e) => setForm({ ...form, user_name: e.target.value })} required />
            </Field>
            <Field label="中文名">
              <Input value={form.user_name_cn} onChange={(e) => setForm({ ...form, user_name_cn: e.target.value })} required />
            </Field>
            <Field label="邮箱">
              <Input type="email" value={form.user_email} onChange={(e) => setForm({ ...form, user_email: e.target.value })} required />
            </Field>
            <Field label="类型">
              <Input type="number" value={form.user_type} onChange={(e) => setForm({ ...form, user_type: e.target.value })} />
            </Field>
            {!editingID ? <Field label="初始密码"><Input type="password" minLength={12} value={form.initial_password} onChange={(e) => setForm({ ...form, initial_password: e.target.value })} required /></Field> : null}
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
          <CardTitle>用户列表</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>用户名</TableHead>
                <TableHead>中文名</TableHead>
                <TableHead>邮箱</TableHead>
                <TableHead className="w-24">类型</TableHead>
                <TableHead className="w-48">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow><TableCell colSpan={5}>加载中...</TableCell></TableRow>
              ) : items.length === 0 ? (
                <TableRow><TableCell colSpan={5}>暂无用户</TableCell></TableRow>
              ) : items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.user_name}</TableCell>
                  <TableCell>{item.user_name_cn}</TableCell>
                  <TableCell className="font-mono text-xs">{item.user_email}</TableCell>
                  <TableCell>{item.user_type}</TableCell>
                  <TableCell>
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => edit(item)}><Pencil className="h-3.5 w-3.5" />编辑</Button>
                      <Button size="sm" variant="outline" onClick={() => resetPassword(item.id)}><RotateCcw className="h-3.5 w-3.5" />密码</Button>
                      <Button size="sm" variant="destructive" onClick={() => remove(item.id)}><Trash2 className="h-3.5 w-3.5" />删除</Button>
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
