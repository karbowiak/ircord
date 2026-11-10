<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public Google readers: Iframely plugins/domains/google.com/docs.google.com.js
// (MIT, Copyright 2013–2022 Itteco Software Corp.; see the backend license notice).
export const embedProvider = {
  id: 'google-documents',
  priority: 100,
  matches: [
    {
      hosts: ['docs.google.com'],
      paths: [
        '/document/d/**',
        '/document/u/*/d/**',
        '/a/*/document/d/**',
        '/a/*/document/u/*/d/**',
        '/spreadsheets/d/**',
        '/spreadsheets/u/*/d/**',
        '/a/*/spreadsheets/d/**',
        '/a/*/spreadsheets/u/*/d/**',
        '/presentation/d/**',
        '/presentation/u/*/d/**',
        '/a/*/presentation/d/**',
        '/a/*/presentation/u/*/d/**',
        '/forms/d/**',
        '/forms/u/*/d/**',
        '/a/*/forms/d/**',
        '/a/*/forms/u/*/d/**',
        '/drawings/d/**',
        '/drawings/u/*/d/**',
        '/a/*/drawings/d/**',
        '/a/*/drawings/u/*/d/**',
      ],
    },
    {
      hosts: ['drive.google.com'],
      paths: ['/file/d/**', '/file/u/*/d/**', '/a/*/file/d/**', '/a/*/file/u/*/d/**'],
    },
    { hosts: ['drive.google.com'], paths: ['/open'], query: ['id'] },
  ],
  layout: { ratio: [3, 4] },
  variants: [
    {
      matches: [
        {
          hosts: ['docs.google.com'],
          paths: [
            '/presentation/d/**',
            '/presentation/u/*/d/**',
            '/a/*/presentation/d/**',
            '/a/*/presentation/u/*/d/**',
          ],
        },
      ],
      layout: { ratio: [16, 9] },
    },
    {
      matches: [
        {
          hosts: ['docs.google.com'],
          paths: [
            '/spreadsheets/d/**',
            '/spreadsheets/u/*/d/**',
            '/a/*/spreadsheets/d/**',
            '/a/*/spreadsheets/u/*/d/**',
            '/drawings/d/**',
            '/drawings/u/*/d/**',
            '/a/*/drawings/d/**',
            '/a/*/drawings/u/*/d/**',
          ],
        },
      ],
      layout: { ratio: [4, 3] },
    },
    {
      matches: [
        {
          hosts: ['docs.google.com'],
          paths: ['/forms/d/**', '/forms/u/*/d/**', '/a/*/forms/d/**', '/a/*/forms/u/*/d/**'],
        },
      ],
      layout: { height: 640 },
    },
  ],
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    {
      hosts: ['docs.google.com'],
      paths: [
        '/document/d/*/preview',
        '/document/d/*/pub',
        '/document/d/e/*/pub',
        '/document/d/*/mobilebasic',
        '/document/d/*/htmlview',
        '/spreadsheets/d/*/preview',
        '/spreadsheets/d/*/pubhtml',
        '/spreadsheets/d/e/*/pubhtml',
        '/spreadsheets/d/*/htmlview',
        '/spreadsheets/d/e/*/htmlview',
        '/presentation/d/*/embed',
        '/presentation/d/e/*/embed',
        '/forms/d/*/viewform',
        '/forms/d/e/*/viewform',
        '/drawings/d/*/preview',
        '/drawings/d/*/pub',
        '/drawings/d/e/*/pub',
      ],
    },
    { hosts: ['drive.google.com'], paths: ['/file/d/*/preview'] },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
