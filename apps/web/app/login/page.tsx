"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { apiFetch } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [userName, setUserName] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      await apiFetch<unknown>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ user_name: userName, password })
      });
      router.push("/projects");
      router.refresh();
    } catch (err: any) {
      setError(err?.message || "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-[calc(100vh-8rem)] max-w-md items-center">
      <Card className="w-full">
        <CardHeader>
          <CardTitle>登录 Fluxa</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={submit}>
            <div className="space-y-2">
              <Label>用户名</Label>
              <Input value={userName} onChange={(event) => setUserName(event.target.value)} autoComplete="username" />
            </div>
            <div className="space-y-2">
              <Label>密码</Label>
              <Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" />
            </div>
            {error ? <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div> : null}
            <Button className="w-full" type="submit" disabled={loading}>
              {loading ? "登录中" : "登录"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
