import './globals.css';
import type { Metadata } from 'next';
import AppNav from '@/components/AppNav';

export const metadata: Metadata = {
  title: 'Knowledge System Dashboard',
  description: 'Code knowledge graph + vector atlas dashboard',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      {/* Flex column: the shared nav takes a fixed strip and the routed viewer
          fills the rest. The viewers size against this height:100% content
          area (not the raw viewport), so adding the nav does not overflow
          their layouts. */}
      <body>
        <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
          <AppNav />
          <div style={{ flex: 1, minHeight: 0, position: 'relative' }}>{children}</div>
        </div>
      </body>
    </html>
  );
}
