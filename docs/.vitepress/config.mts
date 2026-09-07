import { defineConfig } from 'vitepress';

export default defineConfig({
  title: 'BFFX',
  description: 'The High-Performance Go Backend Engine with Universal Realtime, gRPC & Mobile SDKs',
  ignoreDeadLinks: true,
  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['meta', { name: 'theme-color', content: '#3b82f6' }],
  ],
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'BFFX',
    nav: [
      { text: 'Guide', link: '/getting-started/quickstart' },
      { text: 'Core Concepts', link: '/core-concepts/architecture' },
      { text: 'Batteries', link: '/batteries/index' },
      { text: 'Mobile & Web', link: '/mobile/flutter' },
      { text: 'Use Cases', link: '/use-cases/headless-cms-wordpress-alternative' },
      { text: 'CLI Reference', link: '/reference/cli' },
      { text: 'Audit', link: '/release-readiness/BFFX_CORE_AUDIT' },
    ],
    sidebar: [
      {
        text: 'Getting Started',
        collapsed: false,
        items: [
          { text: 'Quickstart Guide', link: '/getting-started/quickstart' },
          { text: 'State of the Project', link: '/getting-started/state_of_the_project' },
          { text: 'Database & Layout Migration', link: '/getting-started/migration' },
        ],
      },
      {
        text: 'Core Concepts',
        collapsed: false,
        items: [
          { text: 'System Architecture', link: '/core-concepts/architecture' },
          { text: 'Architecture Contracts', link: '/core-concepts/architecture_contracts' },
          { text: 'Project Manifest', link: '/core-concepts/project_manifest' },
          { text: 'Resource Manifest', link: '/core-concepts/resource_manifest' },
          { text: 'Admin Manifest', link: '/core-concepts/admin_manifest' },
          { text: 'Database Topology', link: '/core-concepts/database_topology' },
          { text: 'Cache Contract', link: '/core-concepts/cache_contract' },
          { text: 'Observability Contract', link: '/core-concepts/observability_contract' },
        ],
      },
      {
        text: 'Batteries & Addons',
        collapsed: false,
        items: [
          { text: 'Batteries Overview', link: '/batteries/index' },
          { text: 'PostgreSQL Datastore', link: '/batteries/postgres' },
        ],
      },
      {
        text: 'Client & Mobile Integration',
        collapsed: false,
        items: [
          { text: 'Flutter (Dart) SDK', link: '/mobile/flutter' },
          { text: 'REST Client & Handshake', link: '/mobile/rest_client' },
        ],
      },
      {
        text: 'Topical Guides',
        collapsed: true,
        items: [
          { text: 'Authentication & Sessions', link: '/guides/auth' },
          { text: 'Caching Engine & Tags', link: '/guides/caching' },
          { text: 'Testing & Blueprints', link: '/guides/testing' },
          { text: 'Security Inventory', link: '/guides/security_inventory' },
          { text: 'Action Authorization', link: '/guides/action_authorization' },
          { text: 'Production Operations', link: '/guides/operations' },
        ],
      },
      {
        text: 'Use Cases',
        collapsed: true,
        items: [
          { text: 'Use Cases Index', link: '/use-cases/README' },
          { text: 'Headless CMS (WordPress Alternative)', link: '/use-cases/headless-cms-wordpress-alternative' },
        ],
      },
      {
        text: 'Operations & Scaling',
        collapsed: true,
        items: [
          { text: 'Production Runbook', link: '/operations/runbook' },
          { text: 'Horizontal Scaling Checklist', link: '/operations/scaling' },
          { text: 'Audit & Launch Readiness', link: '/release-readiness/BFFX_CORE_AUDIT' },
        ],
      },
      {
        text: 'Reference',
        collapsed: false,
        items: [
          { text: 'CLI Commands Reference', link: '/reference/cli' },
          { text: 'Documentation Map', link: '/README' },
        ],
      },
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/hangry-coder/bffx' },
    ],
    search: {
      provider: 'local',
    },
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 BFFX Authors',
    },
  },
});
