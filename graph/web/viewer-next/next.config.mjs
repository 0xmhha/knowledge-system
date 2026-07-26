/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: false,
  output: 'export',
  images: { unoptimized: true },
  trailingSlash: true,
  // Stop Next.js from 308-redirecting `/api/foo` → `/api/foo/`. In dev mode
  // the rewrite below proxies /api/* to the Go backend, but the redirect
  // fires BEFORE the rewrite — and the redirected URL goes through the
  // rewrite again, hitting the backend which serves /api/foo (no slash) and
  // 308-redirects back. Net result: browser sees ERR_TOO_MANY_REDIRECTS and
  // every API call fails (`fetch('/api/nodes/top')` etc.). We still want
  // trailingSlash for static export folder structure; this only disables
  // the dev-time auto-redirect for paths that don't match the convention.
  skipTrailingSlashRedirect: true,
  // Dev-only proxies (ignored by the static-export build; a production host
  // serves the static files and proxies these paths itself):
  //   /api/*                     → the graph backend (ckg serve, :8080)
  //   /query, /config, /data/*   → the Atlas backend (vector serve.py, :8098)
  // The Atlas endpoints keep the paths the ported viewer already uses; they
  // don't collide with the graph app, which only calls /api/*.
  async rewrites() {
    const atlas = process.env.ATLAS_BACKEND || 'http://localhost:8098';
    return [
      { source: '/api/:path*', destination: 'http://localhost:8080/api/:path*' },
      { source: '/query', destination: `${atlas}/query` },
      { source: '/config', destination: `${atlas}/config` },
      { source: '/data/:path*', destination: `${atlas}/data/:path*` },
    ];
  },
};

export default nextConfig;
