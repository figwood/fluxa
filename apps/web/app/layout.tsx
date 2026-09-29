import './globals.css';
import type { Metadata } from 'next';
import React from 'react';
import { AppShell } from '@/components/AppShell';

export const metadata: Metadata = {
  title: 'Fluxa',
  description: 'Delivery flow skeleton MVP'
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body className="min-h-screen antialiased">
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}
