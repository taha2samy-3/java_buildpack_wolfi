import { releases } from '../lib/site.js';

export function GET() {
  const feed = releases.map(({ tag, version, url, published, prerelease }) => ({ tag, version, url, published, prerelease }));
  return new Response(JSON.stringify(feed, null, 2), { headers: { 'Content-Type': 'application/json' } });
}
