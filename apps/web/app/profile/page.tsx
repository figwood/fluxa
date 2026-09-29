"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, clearToken, putJSON } from "@/lib/api";
import type { User } from "@/lib/types";
import { Field } from "@/components/Field";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function ProfilePage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    apiFetch<User>("/auth/profile").then(setUser).catch((err) => setError(err.message || "个人资料加载失败"));
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");
    if (newPassword !== confirmPassword) {
      setError("两次输入的新密码不一致");
      return;
    }
    setLoading(true);
    try {
      await putJSON<void>("/auth/password", { current_password: currentPassword, new_password: newPassword });
      setSuccess("密码已修改，请使用新密码重新登录。");
      await clearToken();
      router.push("/login");
    } catch (err: any) {
      setError(err.message || "密码修改失败");
    } finally {
      setLoading(false);
    }
  }

  return <div className="grid w-full gap-5">
    <div>
      <h1 className="text-2xl font-semibold tracking-normal">个人资料</h1>
      <p className="mt-1 text-sm text-muted-foreground">查看帐户信息并管理登录密码。</p>
    </div>
    {error ? <div role="alert" className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div> : null}
    {success ? <div role="status" className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{success}</div> : null}
    <Card>
      <CardHeader><CardTitle>帐户信息</CardTitle><CardDescription>当前登录帐户的基本资料。</CardDescription></CardHeader>
      <CardContent className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <ProfileValue label="用户名" value={user?.user_name} />
        <ProfileValue label="姓名" value={user?.user_name_cn} />
        <ProfileValue label="邮箱" value={user?.user_email} />
        <ProfileValue label="用户类型" value={user ? String(user.user_type) : undefined} />
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle>修改密码</CardTitle><CardDescription>新密码至少 8 位。修改后需要重新登录。</CardDescription></CardHeader>
      <CardContent>
        <form className="grid gap-4 md:grid-cols-2 xl:grid-cols-4" onSubmit={submit}>
          <Field label="当前密码"><Input type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} required /></Field>
          <Field label="新密码"><Input type="password" autoComplete="new-password" minLength={8} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} required /></Field>
          <Field label="确认新密码"><Input type="password" autoComplete="new-password" minLength={8} value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} required /></Field>
          <Button type="submit" className="w-fit md:col-span-2 xl:col-span-4" disabled={loading}>{loading ? "正在修改..." : "修改密码"}</Button>
        </form>
      </CardContent>
    </Card>
  </div>;
}

function ProfileValue({ label, value }: { label: string; value?: string }) {
  return <div className="grid gap-1"><Label>{label}</Label><div className="min-h-9 rounded-md border bg-muted/40 px-3 py-2 text-sm">{value || "加载中..."}</div></div>;
}
