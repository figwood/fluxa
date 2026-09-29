"use client";

import React, { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Plus, X } from "lucide-react";
import { apiFetch, postJSON } from "@/lib/api";
import type { Project } from "@/lib/types";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

type ProjectForm = { name: string; key: string; description: string };
const emptyForm: ProjectForm = { name: "", key: "", description: "" };

export default function ProjectsPage() {
  const router = useRouter();
  const [items, setItems] = useState<Project[]>([]);
  const [form, setForm] = useState<ProjectForm>(emptyForm);
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await apiFetch<Project[]>("/projects"));
    } catch (err: any) {
      setError(err.message || "加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const create = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    setCreating(true);
    try {
      const created=await postJSON<Project>("/projects", form);
      setForm(emptyForm);
      setCreateOpen(false);
      router.push(`/projects/${created.id}`);
    } catch (err: any) {
      setError(err.message || "创建失败");
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="grid gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">项目</h1>
          <p className="mt-1 text-sm text-muted-foreground">业务项目是成员、服务、任务和发布的统一上下文。</p>
        </div>
        <Button onClick={() => setCreateOpen(true)}><Plus className="h-4 w-4" />创建项目</Button>
      </div>
      {error ? <Alert>{error}</Alert> : null}
      <Card>
        <CardContent className="pt-5">
          <Table>
            <TableHeader><TableRow><TableHead>项目名称</TableHead><TableHead className="w-48">Key</TableHead><TableHead>描述</TableHead></TableRow></TableHeader>
            <TableBody>
              {loading ? <TableRow><TableCell colSpan={3}>加载中...</TableCell></TableRow> : items.length === 0 ? <TableRow><TableCell colSpan={3}>暂无项目</TableCell></TableRow> : items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">
                    <Link href={`/projects/${item.id}`} className="text-primary hover:underline">{item.name}</Link>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{item.key}</TableCell>
                  <TableCell className="text-muted-foreground">{item.description || "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      {createOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/30 p-4">
          <div className="w-full max-w-lg rounded-lg border border-border bg-card p-5 shadow-lg">
            <div className="mb-4 flex items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold">创建项目</h2>
                <p className="mt-1 text-sm text-muted-foreground">填写项目基本信息，创建后进入项目详情页。</p>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setCreateOpen(false)}
                aria-label="关闭创建项目弹窗"
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
            <form className="grid gap-4" onSubmit={create}>
              <Field label="项目名称"><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
              <Field label="Key"><Input value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} required /></Field>
              <Field label="描述"><Input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></Field>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setCreateOpen(false)} disabled={creating}>取消</Button>
                <Button type="submit" disabled={creating}>{creating ? "创建中..." : "确定"}</Button>
              </div>
            </form>
          </div>
        </div>
      ) : null}
    </div>
  );
}
