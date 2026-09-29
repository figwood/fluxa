"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { CheckSquare, ChevronDown, Cpu, GitBranch, KeyRound, LogOut, Menu, Rocket, Settings, ShieldCheck, Users, X } from "lucide-react";
import { ProjectScopeProvider, projectScopeStorageKey } from "@/components/ProjectScope";
import { apiFetch, clearToken } from "@/lib/api";
import type { Project, UserAccess } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";

const adminItems = [
  { href: "/admin/tasks", label: "全局任务", icon: CheckSquare },
  { href: "/admin/releases", label: "全局发布", icon: Rocket },
  { href: "/admin/workflows", label: "工作流", icon: GitBranch },
  { href: "/admin/workers", label: "Workers", icon: Cpu },
  { href: "/admin/users", label: "用户", icon: Users },
  { href: "/admin/roles", label: "角色", icon: ShieldCheck },
  { href: "/admin/privileges", label: "权限", icon: KeyRound }
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isLogin = pathname === "/login";
  const [isAdmin, setIsAdmin] = useState<boolean | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectIdState] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const pathProjectId = useMemo(() => pathname.match(/^\/projects\/([^/]+)/)?.[1] || "", [pathname]);
  const selectedProject = projects.find((item) => item.id === selectedProjectId);

  useEffect(() => {
    if (isLogin) return;
    apiFetch<UserAccess>("/auth/me").then((access) => {
      const admin = access.global_roles.includes("admin");
      setIsAdmin(admin);
      if (pathname.startsWith("/admin") && !admin) router.replace("/");
    }).catch(() => setIsAdmin(false));
  }, [isLogin, pathname, router]);

  useEffect(() => {
    if (isLogin) return;
    apiFetch<Project[]>("/projects").then((rows) => {
      setProjects(rows);
      const stored = window.localStorage.getItem(projectScopeStorageKey) || "";
      const next = pathProjectId || (stored && rows.some((item) => item.id === stored) ? stored : "") || rows[0]?.id || "";
      setSelectedProjectIdState(next);
      if (next) window.localStorage.setItem(projectScopeStorageKey, next);
    }).catch(() => setProjects([]));
  }, [isLogin, pathProjectId]);

  useEffect(() => setMobileOpen(false), [pathname]);

  const setSelectedProjectId = (nextProjectId: string) => {
    setSelectedProjectIdState(nextProjectId);
    window.localStorage.setItem(projectScopeStorageKey, nextProjectId);
    if (pathProjectId) router.push(pathname.replace(`/projects/${pathProjectId}`, `/projects/${nextProjectId}`));
  };

  const mainLink = (href: string, label: string, Icon: React.ElementType) => {
    const active = pathname === href || pathname.startsWith(`${href}/`) || (href === "/tasks" && pathname.includes("/tasks")) || (href === "/releases" && pathname.includes("/releases"));
    return <Link href={href} className={cn("inline-flex h-10 items-center gap-2 rounded-lg px-4 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground", active && "bg-primary/10 text-primary")}><Icon className="h-4 w-4" />{label}</Link>;
  };

  const adminLink = (item: { href: string; label: string; icon: React.ElementType }) => {
    const Icon = item.icon;
    return <Link key={item.href} href={item.href} className={cn("flex h-9 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground hover:bg-muted hover:text-foreground", pathname.startsWith(item.href) && "bg-primary/10 font-medium text-primary")}><Icon className="h-4 w-4" />{item.label}</Link>;
  };

  return <ProjectScopeProvider value={{ selectedProjectId, setSelectedProjectId }}>
    <div className="min-h-screen bg-background">
      {!isLogin ? <header className="sticky top-0 z-30 border-b bg-card/95 backdrop-blur">
        <div className="flex h-16 items-center gap-4 px-4 md:px-6">
          <Link href="/" className="flex shrink-0 items-center gap-3"><div className="grid h-9 w-9 place-items-center rounded-lg bg-primary text-primary-foreground"><Rocket className="h-5 w-5" /></div><div className="hidden sm:block"><div className="text-base font-semibold">Fluxa</div><div className="text-xs text-muted-foreground">让开发与发布更清晰</div></div></Link>
          <nav className="hidden flex-1 items-center gap-1 md:flex">{mainLink("/tasks", "开发任务", CheckSquare)}{mainLink("/releases", "发布管理", Rocket)}</nav>
          <div className="ml-auto flex min-w-0 items-center gap-2">
            <span className="shrink-0 text-sm text-muted-foreground">当前项目</span>
            <Select className="w-36 sm:w-52" aria-label="切换项目" value={selectedProjectId} onChange={(event) => setSelectedProjectId(event.target.value)} options={projects.map((item) => ({ label: item.name, value: item.id }))} disabled={projects.length === 0} />
            <details className="group relative hidden md:block">
              <summary className="flex h-9 cursor-pointer list-none items-center gap-1 rounded-md px-3 text-sm font-medium text-muted-foreground hover:bg-muted"><Settings className="h-4 w-4" />设置<ChevronDown className="h-3.5 w-3.5" /></summary>
              <div className="absolute right-0 mt-2 grid w-52 gap-1 rounded-lg border bg-card p-2 shadow-lg">
                <div className="px-2 py-1 text-xs text-muted-foreground">{selectedProject?.name || "当前项目"}</div>
                {selectedProjectId ? <><Link className="rounded-md px-2 py-2 text-sm hover:bg-muted" href={`/projects/${selectedProjectId}`}>项目概览与设置</Link><Link className="rounded-md px-2 py-2 text-sm hover:bg-muted" href={`/projects/${selectedProjectId}/gitlab`}>GitLab 与发布服务</Link><Link className="rounded-md px-2 py-2 text-sm hover:bg-muted" href={`/projects/${selectedProjectId}/members`}>项目成员</Link></> : null}
                {isAdmin ? <Link className="rounded-md border-t px-2 py-2 text-sm hover:bg-muted" href="/admin">平台管理</Link> : null}
                <button className="flex items-center gap-2 rounded-md px-2 py-2 text-left text-sm text-muted-foreground hover:bg-muted" onClick={async () => { await clearToken(); router.push("/login"); router.refresh(); }}><LogOut className="h-4 w-4" />退出登录</button>
              </div>
            </details>
            <Button variant="ghost" size="sm" className="md:hidden" onClick={() => setMobileOpen((value) => !value)} aria-label="打开导航">{mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}</Button>
          </div>
        </div>
        {mobileOpen ? <div className="grid gap-1 border-t p-3 md:hidden">{mainLink("/tasks", "开发任务", CheckSquare)}{mainLink("/releases", "发布管理", Rocket)}{selectedProjectId ? <Link className="rounded-lg px-4 py-2 text-sm text-muted-foreground" href={`/projects/${selectedProjectId}`}>当前项目 · 项目设置</Link> : null}{isAdmin ? <Link className="rounded-lg px-4 py-2 text-sm text-muted-foreground" href="/admin">平台管理</Link> : null}<button className="rounded-lg px-4 py-2 text-left text-sm text-muted-foreground" onClick={async () => { await clearToken(); router.push("/login"); }}>退出登录</button></div> : null}
      </header> : null}
      <div className={cn(pathname.startsWith("/admin") && "md:grid md:grid-cols-[220px_1fr]")}>
        {pathname.startsWith("/admin") && isAdmin === true ? <aside className="hidden min-h-[calc(100vh-64px)] border-r bg-card p-4 md:block"><div className="mb-3 px-3 text-xs font-semibold uppercase text-muted-foreground">平台管理</div><nav className="grid gap-1">{adminItems.map(adminLink)}</nav></aside> : null}
        <main className="min-w-0 px-4 py-6 md:px-6 md:py-8">{pathname.startsWith("/admin") && isAdmin !== true ? <div className="py-16 text-center text-sm text-muted-foreground">正在验证管理员权限...</div> : children}</main>
      </div>
    </div>
  </ProjectScopeProvider>;
}
