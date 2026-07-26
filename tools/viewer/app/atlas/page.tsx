import Atlas from '@/components/Atlas';

// The vector Atlas viewer, ported from vector/web/viewer/index.html into this
// app as a client component. It fetches its data from the Atlas backend
// (serve.py) via the /query, /config, /data/* endpoints, proxied by the dev
// rewrites in next.config.mjs.
export default function AtlasPage() {
  return <Atlas />;
}
