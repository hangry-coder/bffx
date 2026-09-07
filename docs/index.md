---
layout: home

hero:
  name: "BFFX"
  text: "High-Performance Backend Engine for Go, Mobile & Web"
  tagline: "Single-binary simplicity with PocketBase developer joy, enterprise PostgreSQL power, and native Dart & TypeScript client generation."
  actions:
    - theme: brand
      text: Quickstart Guide
      link: /getting-started/quickstart
    - theme: alt
      text: Architecture Overview
      link: /core-concepts/architecture
    - theme: alt
      text: GitHub Repository
      link: https://github.com/hangry-coder/bffx

features:
  - icon: 🚀
    title: Single Binary Simplicity
    details: Deploy as a single self-contained Go binary with zero external runtime dependencies. Runs locally on embedded SQLite and scales to distributed Postgres with a single flag.
  - icon: ⚡
    title: Universal Realtime & Streams
    details: Native Server-Sent Events (SSE) streaming, pub/sub event bus (in-memory or Redis), and instant client-side topic subscription for Flutter & Web.
  - icon: 📱
    title: Generated Type-Safe SDKs
    details: Complete Flutter (Dart) and Web (TypeScript) client SDKs generated on every build, featuring typed CRUD, auth session management, and SSE subscriptions.
  - icon: 📄
    title: OpenAPI 3.1 & gRPC Dual Engine
    details: Automatic OpenAPI 3.1 exporter for frontend teams alongside high-throughput Protobuf & gRPC transport for mobile apps.
  - icon: 🛡️
    title: Enterprise Security & RBAC
    details: Refresh token rotation, role-based access control, IP/email brute-force protection, rate limiting, and automated security verification out of the box.
  - icon: 🤖
    title: AI & MCP Agent Ready
    details: Built-in Model Context Protocol (MCP) server allowing AI agents (Claude, Gemini) to inspect schemas, query routes, and safely assist during development.
---
