import { readFileSync } from 'node:fs';
import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';

const data = JSON.parse(readFileSync(new URL('./src/data/buildpack.json', import.meta.url), 'utf8'));

function placeholders() {
  const values = { '[[latest]]': data.latest, '[[registry]]': data.registry, '[[repository]]': data.repository };
  const walk = (node) => {
    if (typeof node.value === 'string') {
      for (const [key, value] of Object.entries(values)) {
        node.value = node.value.replaceAll(key, value);
      }
    }
    if (typeof node.url === 'string') {
      for (const [key, value] of Object.entries(values)) {
        node.url = node.url.replaceAll(key, value);
      }
    }
    node.children?.forEach(walk);
  };
  return (tree) => walk(tree);
}

export default defineConfig({
  site: process.env.SITE_URL ?? 'https://taha2samy-3.github.io',
  base: process.env.SITE_BASE ?? '/',
  trailingSlash: 'ignore',
  integrations: [mdx()],
  markdown: {
    syntaxHighlight: 'prism',
    remarkPlugins: [placeholders],
  },
  security: {
    csp: {
      directives: [
        "default-src 'self'",
        "img-src 'self' data:",
        "font-src 'self'",
        "connect-src 'self'",
        "object-src 'none'",
        "base-uri 'none'",
        "form-action 'none'",
      ],
    },
  },
});
