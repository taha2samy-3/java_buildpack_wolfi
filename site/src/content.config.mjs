import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { z } from 'astro/zod';

const docs = defineCollection({
  loader: glob({ pattern: '*.mdx', base: './src/content/docs' }),
  schema: z.object({
    title: z.string(),
    nav: z.string().optional(),
    description: z.string(),
    order: z.number(),
    image: z.string().optional(),
  }),
});

export const collections = { docs };
