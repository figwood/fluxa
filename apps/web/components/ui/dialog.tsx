"use client";

import React, { useEffect } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export function Dialog({ open, onOpenChange, children, className }: { open: boolean; onOpenChange: (open: boolean) => void; children: React.ReactNode; className?: string }) {
  useEffect(() => {
    if (!open) return;
    const close = (event: KeyboardEvent) => event.key === "Escape" && onOpenChange(false);
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [open, onOpenChange]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-foreground/35 p-4" onMouseDown={(event) => event.target === event.currentTarget && onOpenChange(false)}>
      <div role="dialog" aria-modal="true" className={cn("max-h-[90vh] w-full max-w-lg overflow-auto rounded-xl border bg-card shadow-xl", className)}>
        {children}
      </div>
    </div>
  );
}

export function DialogHeader({ title, description, onClose }: { title: string; description?: string; onClose: () => void }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b p-5">
      <div><h2 className="text-lg font-semibold">{title}</h2>{description ? <p className="mt-1 text-sm text-muted-foreground">{description}</p> : null}</div>
      <Button type="button" variant="ghost" size="sm" onClick={onClose} aria-label="关闭"><X className="h-4 w-4" /></Button>
    </div>
  );
}
