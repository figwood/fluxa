"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Activity, CheckCircle2, Clock3, RefreshCw, Save, Server, XCircle } from "lucide-react";
import { apiFetch, patchJSON } from "@/lib/api";
import type { WorkerHeartbeat, WorkerJobSnapshot, WorkerSnapshot } from "@/lib/types";
import { Alert } from "@/components/ui/alert";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function statusVariant(status: string): BadgeProps["variant"] {
  if (status === "success" || status === "idle") return "success";
  if (status === "failed" || status === "offline") return "danger";
  if (status === "pending") return "warning";
  if (status === "running") return "info";
  return "secondary";
}

function formatTime(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(new Date(value));
}

function formatInterval(ms: number) {
  if (!ms) return "-";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(ms % 1000 === 0 ? 0 : 1)} s`;
}

export default function WorkersPage() {
  const [snapshot, setSnapshot] = useState<WorkerSnapshot | null>(null);
  const [desired, setDesired] = useState("1");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await apiFetch<WorkerSnapshot>("/workers");
      setSnapshot(data);
      setDesired(String(data.config.desired_replicas));
    } catch (err: any) {
      setError(err.message || "加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    const timer = window.setInterval(() => {
      load().catch(() => undefined);
    }, 5000);
    return () => window.clearInterval(timer);
  }, []);

  const saveDesired = async () => {
    const replicas = Number.parseInt(desired, 10);
    if (!Number.isFinite(replicas)) {
      setError("worker 个数必须是数字");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const data = await patchJSON<WorkerSnapshot>("/workers/config", { desired_replicas: replicas });
      setSnapshot(data);
      setDesired(String(data.config.desired_replicas));
    } catch (err: any) {
      setError(err.message || "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const activeWorkers = useMemo(() => snapshot?.workers.filter((item) => !item.stale) || [], [snapshot?.workers]);
  const displayedWorkers = snapshot?.workers || [];
  const desiredCount = snapshot?.config.desired_replicas || Number.parseInt(desired, 10) || 1;
  const runningWorkers = activeWorkers.filter((item) => item.status === "running").length;
  const plannedSlots = Math.max(0, desiredCount - activeWorkers.length);

  return (
    <div className="grid gap-5">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Workers</h1>
          <p className="mt-1 text-sm text-muted-foreground">查看 Worker 配置、心跳、运行任务和发布队列。</p>
        </div>
        <Button variant="outline" onClick={load} disabled={loading}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      <div className="grid gap-4 lg:grid-cols-[360px_1fr]">
        <Card>
          <CardHeader><CardTitle>运行配置</CardTitle></CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-2">
              <div className="text-xs font-medium uppercase text-muted-foreground">期望 Worker 个数</div>
              <div className="flex gap-2">
                <Input
                  min={1}
                  max={20}
                  type="number"
                  value={desired}
                  onChange={(event) => setDesired(event.target.value)}
                />
                <Button onClick={saveDesired} disabled={saving}>
                  <Save className="h-4 w-4" />
                  保存
                </Button>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3 text-sm">
              <Metric label="执行模式" value={snapshot?.config.mode || "-"} />
              <Metric label="轮询间隔" value={formatInterval(snapshot?.config.poll_interval_ms || 0)} />
              <Metric label="在线 Worker" value={String(activeWorkers.length)} />
              <Metric label="运行中" value={String(runningWorkers)} />
            </div>
            <div className="rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
              期望副本数保存在控制平面；实际执行实例以 worker 进程心跳为准。
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <Stat icon={Clock3} label="Pending Jobs" value={snapshot?.stats.pending || 0} tone="warning" />
          <Stat icon={Activity} label="Running Jobs" value={snapshot?.stats.running || 0} tone="info" />
          <Stat icon={CheckCircle2} label="Success Jobs" value={snapshot?.stats.success || 0} tone="success" />
          <Stat icon={XCircle} label="Failed Jobs" value={snapshot?.stats.failed || 0} tone="danger" />
        </div>
      </div>

      <Card>
        <CardHeader><CardTitle>Worker 实例</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Worker</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>配置</TableHead>
                <TableHead>当前任务</TableHead>
                <TableHead>最后心跳</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && !snapshot ? (
                <TableRow><TableCell colSpan={5}>加载中...</TableCell></TableRow>
              ) : displayedWorkers.length === 0 && plannedSlots === 0 ? (
                <TableRow><TableCell colSpan={5}>暂无 Worker 心跳</TableCell></TableRow>
              ) : (
                <>
                  {displayedWorkers.map((worker) => <WorkerRow key={worker.id} worker={worker} />)}
                  {Array.from({ length: plannedSlots }).map((_, index) => (
                    <PlannedWorkerRow key={`planned-${index}`} index={activeWorkers.length + index + 1} />
                  ))}
                </>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>最近任务分配</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Job</TableHead>
                <TableHead>发布单</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>Worker</TableHead>
                <TableHead>时间</TableHead>
                <TableHead>消息</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {!snapshot || snapshot.jobs.length === 0 ? (
                <TableRow><TableCell colSpan={6}>暂无发布任务</TableCell></TableRow>
              ) : snapshot.jobs.map((job) => <JobRow key={job.id} job={job} />)}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border bg-background p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 font-medium">{value}</div>
    </div>
  );
}

function Stat({ icon: Icon, label, value, tone }: { icon: React.ElementType; label: string; value: number; tone: "warning" | "info" | "success" | "danger" }) {
  const color = {
    warning: "text-amber-700",
    info: "text-sky-700",
    success: "text-emerald-700",
    danger: "text-rose-700"
  }[tone];
  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-4 pt-5">
        <div>
          <div className="text-xs font-medium uppercase text-muted-foreground">{label}</div>
          <div className="mt-2 text-2xl font-semibold">{value}</div>
        </div>
        <div className="grid h-10 w-10 place-items-center rounded-md bg-muted">
          <Icon className={`h-5 w-5 ${color}`} />
        </div>
      </CardContent>
    </Card>
  );
}

function WorkerRow({ worker }: { worker: WorkerHeartbeat }) {
  const task = worker.current_job_id ? (
    <div className="grid gap-1">
      <span className="font-mono text-xs">{worker.current_job_id}</span>
      <Link className="text-xs text-primary" href={`/releases/${worker.current_release_id}`}>{worker.current_release_id}</Link>
    </div>
  ) : "-";

  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-2">
          <Server className="h-4 w-4 text-muted-foreground" />
          <div className="min-w-0">
            <div className="truncate font-mono text-xs">{worker.id}</div>
            <div className="truncate text-xs text-muted-foreground">{worker.hostname || "-"}</div>
          </div>
        </div>
      </TableCell>
      <TableCell><Badge variant={statusVariant(worker.status)}>{worker.status}</Badge></TableCell>
      <TableCell>
        <div className="text-xs text-muted-foreground">
          <div>{worker.mode}</div>
          <div>{formatInterval(worker.poll_interval_ms)}</div>
        </div>
      </TableCell>
      <TableCell>{task}</TableCell>
      <TableCell className="font-mono text-xs">{formatTime(worker.last_seen_at)}</TableCell>
    </TableRow>
  );
}

function PlannedWorkerRow({ index }: { index: number }) {
  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-2 text-muted-foreground">
          <Server className="h-4 w-4" />
          <span>planned-worker-{index}</span>
        </div>
      </TableCell>
      <TableCell><Badge variant="outline">planned</Badge></TableCell>
      <TableCell className="text-xs text-muted-foreground">等待心跳</TableCell>
      <TableCell>-</TableCell>
      <TableCell>-</TableCell>
    </TableRow>
  );
}

function JobRow({ job }: { job: WorkerJobSnapshot }) {
  return (
    <TableRow>
      <TableCell className="font-mono text-xs">{job.id}</TableCell>
      <TableCell><Link className="text-primary" href={`/releases/${job.release_id}`}>{job.release_id}</Link></TableCell>
      <TableCell><Badge variant={statusVariant(job.status)}>{job.status}</Badge></TableCell>
      <TableCell className="font-mono text-xs">{job.worker_id || "-"}</TableCell>
      <TableCell className="text-xs text-muted-foreground">
        <div>创建：{formatTime(job.created_at)}</div>
        <div>开始：{formatTime(job.started_at)}</div>
        <div>结束：{formatTime(job.finished_at)}</div>
      </TableCell>
      <TableCell className="max-w-[280px] truncate">{job.message || "-"}</TableCell>
    </TableRow>
  );
}
