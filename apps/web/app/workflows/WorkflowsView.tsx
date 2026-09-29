"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { GitBranch, RefreshCw, Settings2 } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { WorkflowConfig } from "@/lib/types";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function WorkflowsView({ projectId = "", adminView = false }: { projectId?: string; adminView?: boolean }) {
  const [items, setItems] = useState<WorkflowConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await apiFetch<WorkflowConfig[]>(projectId ? `/projects/${projectId}/workflows` : "/workflows"));
    } catch (err: any) {
      setError(err.message || "加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="grid gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">{adminView ? "全局工作流" : "工作流"}</h1>
          <p className="mt-1 text-sm text-muted-foreground">任务工作流和发布工作流</p>
        </div>
        <Button variant="outline" onClick={load} disabled={loading}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>
      {error ? <Alert>{error}</Alert> : null}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <GitBranch className="h-5 w-5" />
            工作流配置
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                {adminView ? <TableHead>项目 ID</TableHead> : null}
                <TableHead>类型</TableHead>
                <TableHead>起始状态</TableHead>
                <TableHead>阶段数</TableHead>
                <TableHead>状态数</TableHead>
                <TableHead>流转数</TableHead>
                <TableHead className="w-28">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow><TableCell colSpan={adminView ? 8 : 7}>加载中...</TableCell></TableRow>
              ) : items.length === 0 ? (
                <TableRow><TableCell colSpan={adminView ? 8 : 7}>暂无工作流</TableCell></TableRow>
              ) : items.map((item) => (
                <TableRow key={`${item.project_id}:${item.kind}`}>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  {adminView ? <TableCell className="font-mono text-xs">{item.project_id}</TableCell> : null}
                  <TableCell><Badge variant="outline">{item.kind}</Badge></TableCell>
                  <TableCell className="font-mono text-xs">{item.initial_status}</TableCell>
                  <TableCell>{item.stages.length}</TableCell>
                  <TableCell>{item.statuses.length}</TableCell>
                  <TableCell>{item.transitions.length}</TableCell>
                  <TableCell>
                    <Link href={`/projects/${item.project_id || projectId}/workflows/${item.kind}`}>
                      <Button size="sm" variant="outline">
                        <Settings2 className="h-4 w-4" />
                        编辑
                      </Button>
                    </Link>
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
