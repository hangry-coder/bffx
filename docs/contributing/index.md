---
title: Contributing Guidelines & Maintainer Hub
description: Technical standards, governance overview, and repository health metrics for framework maintainers.
category: contributing
---

# 🤝 Contributing to BFFX: Maintainer Hub

Welcome to the BFFX Developer & Maintainer Hub. This directory hosts standard directives, automation guides, and quality gates for developers expanding, testing, and refining the core framework.

---

## 📋 1. Core Guidelines

If you are a new contributor, please start by reviewing our community standards inside the `.github/` directory:
- **[Contributor Guidelines](../../.github/CONTRIBUTING.md)** — Workflow matrix, PR requirements, and local development setups.
- **[Governance Model](../../.github/GOVERNANCE.md)** — Decision-making process, elevation paths, and technical RFC rules.
- **[Security Policy](../../.github/SECURITY.md)** — Responsible vulnerability disclosure channels.

---

## 🛠️ 2. Maintainer Tooling & Directives

For developers working directly on the Go compiler, routing orchestrator, or MCP control layers:

### 🤖 [AI Directives (Antigravity)](ai_directives.md)
System instructions, security boundaries, and filesystem threat models for Model Context Protocol (MCP) integrations and IDE agents (Cursor, Copilot, Gemini). **Mandatory reading for AI pair programming sessions.**

### 🧪 [Test Coverage Gates](test_coverage.md)
Parity tests benchmarks and E2E coverage thresholds required before passing continuous integration (CI/CD) pipelines.

---

## 💻 3. Technical Stack Standards

All code landed in the core framework must strictly conform to:
1. **Go Parity**: Standardized on **Go 1.26+** syntax.
2. **Context Threading**: Every storage driver, hook, and database query must thread `context.Context` from the HTTP request ingress down to the database connection pool boundary.
3. **Zero-Boilerplate Drift**: All generators (`bffx generate`) and sync compiler processes must be tested against standalone test cases to ensure zero codegen drift.
