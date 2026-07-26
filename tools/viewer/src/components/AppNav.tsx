'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import styles from './AppNav.module.css';

// AppNav is the shared top menu that switches between the two viewers that
// now live in one app: the code-knowledge Graph and the vector Atlas. It is a
// slim bar rendered above the routed page in the root layout, so neither
// full-height viewer has to know about the other.
const TABS = [
  { href: '/', label: 'Graph', match: (p: string) => p === '/' || p === '' },
  { href: '/atlas', label: 'Atlas', match: (p: string) => p.startsWith('/atlas') },
];

export default function AppNav() {
  const pathname = usePathname() ?? '/';
  return (
    <nav className={styles.nav} aria-label="viewer switch">
      <span className={styles.brand}>Knowledge System</span>
      {TABS.map((t) => (
        <Link
          key={t.href}
          href={t.href}
          className={`${styles.tab} ${t.match(pathname) ? styles.active : ''}`}
        >
          {t.label}
        </Link>
      ))}
    </nav>
  );
}
