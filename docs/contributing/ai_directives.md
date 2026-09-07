# BFFX AI Directives (Antigravity)

This document contains permanent directives for AI coding assistants working on the BFFX project. These rules must be adhered to at all times.

## 1. Version Control & Pushing
- **NO DIRECT PUSHES**: Never push directly to the `main` branch or any remote repository unless explicitly requested for a specific task.
- **Commit Locally**: Always stage and commit changes locally.
- **PR Workflow**: Prefer creating feature branches for complex changes.

## 2. Documentation & Planning
- **Walkthroughs in Repo**: Every completed plan (v2.x, etc.) must have a corresponding walkthrough file saved in the `plan/` directory of the repository, not just as an AI artifact.
- **Update CLI Reference**: Any new CLI commands or flags must be immediately documented in `docs/reference/cli.md`.
- **Architecture Integrity**: Keep [`docs/core-concepts/architecture.md`](../../docs/core-concepts/architecture.md) and [`docs/core-concepts/architecture_contracts.md`](../../docs/core-concepts/architecture_contracts.md) updated with major structural changes (including build profile, cache, observability, and taxonomy contracts).
- **System Health**: Ensure **`bffx doctor`** (including **framework / vendoring** and **build profile** checks under `pkg/doctor`) is updated when new dependencies, environment requirements, vendoring metadata, or packaging modes change.
- **AI-Native (MCP)**: Keep the MCP server (`pkg/mcp`) updated with tools that reflect new project capabilities.

## 3. Technical Standards
- **Go Version**: Always use **Go 1.26** (or newer) for all builds, Dockerfiles, and CI/CD pipelines.

## 4. Testing
- **E2E/Integration First**: Before finalizing a deployment-related task, verify it with the integration tests in `tests/integration/`.

## 5. Model Context Protocol (MCP) Threat Model

The Model Context Protocol (MCP) server inside BFFX (`pkg/mcp`) is an AI-native integration designed to allow large language models (LLMs) and agentic IDE extensions (such as Cursor or VS Code) to interact programmatically with the BFFX framework. The MCP server exposes a rich suite of tools for reading manifests, generating scaffolds, triggering compilation/sync routines, running health diagnostics, and orchestrating deployments. 

Because these tools execute local process commands, read filesystem content, and run deployment scripts, securing the MCP server is critical to preventing remote code execution (RCE) and local privilege escalation. This threat model details the architecture, trust boundaries, threat scenarios, and mitigation strategies for the BFFX MCP server.

### 5.1 Architecture & Trust Boundaries

The MCP server supports two main communication channels (transports):
- **Standard Input/Output (stdio)**: This is the default, highly secure transport. The MCP server runs as a subprocess launched directly by the host IDE. Trust is delegated entirely to the local system's process isolation and user permissions.
- **HTTP Server (http)**: The server launches a local web server (using `net/http`) listening for JSON-RPC payloads on `/mcp`. This transport allows external processes or remote agents to interact with the project but introduces a network-exposed trust boundary.

### 5.2 Threat Scenario Analysis

#### Scenario A: External Network Exposure (Bind Address)
- **Threat**: If the HTTP MCP server binds to the wildcard address `0.0.0.0`, any device on the same local network (or public internet if port-forwarded) could access the endpoint. An attacker could list manifests, execute dry-runs, or trigger code generation.
- **BFFX Countermeasure**: The server explicitly binds to the loopback interface `127.0.0.1:%d`. It does not listen on all interfaces. This isolates network access exclusively to the local host. Port-forwarding or reverse-proxy configurations must be explicitly and manually configured by the operator if remote access is required.

#### Scenario B: Unauthenticated Access & Token Bruteforcing
- **Threat**: An unauthorized local process (e.g., a browser sandbox breakout or malicious background script) attempts to connect to `http://127.0.0.1:8081/mcp` and execute commands.
- **BFFX Countermeasure**: The server implements bearer token validation. When the `BFFX_MCP_TOKEN` environment variable or `--token` flag is set, all requests must present a matching `Authorization: Bearer <token>` header. Unauthenticated or malformed requests are rejected with a `401 Unauthorized` status before the request body is parsed or any handlers are invoked.
- **Guideline**: Operators must use a cryptographically strong random string (minimum 32 characters) for `BFFX_MCP_TOKEN`. Avoid easily guessable strings or default values in production-like development environments.

#### Scenario C: Path Traversal via File Operations
- **Threat**: The `manifest.read` tool allows the agent to retrieve filesystem contents. A compromised agent or malicious payload could attempt a directory traversal attack (e.g., `../../../../etc/passwd` or `/Users/user/.ssh/id_rsa`) to exfiltrate secrets.
- **BFFX Countermeasure**: The server enforces strict path validation within `toolManifestRead`. The requested path is joined with the project's root directory, and the server validates that the resulting path maintains the prefix of the project root (`filepath.HasPrefix(fullPath, s.root)`). Any attempt to read files outside the project boundary is blocked with an "access denied" error.

#### Scenario D: Abuse of Administrative Tools (Code Scaffolding & Deployment)
- **Threat**: Tools like `resource.create` and `deploy.ship` execute shell commands (`generator.GenerateScaffold`, `deploy.Ship`) which compile code and create Docker containers. A compromised LLM could write malicious code to a new resource or ship a compromised build.
- **BFFX Countermeasure**: These tools are intended for use inside local interactive development sessions where the user is monitoring output. They should not be exposed on production or shared multi-user environments.

### 5.3 Operational Security Guidelines

1. **Prefer `stdio` Transport**: For standard local development with Cursor/VS Code, always use the default `stdio` transport. It completely bypasses the network stack, removing the remote attack vector.
2. **Restrict Process Permissions**: Run the `bffx mcp` server under a non-root user with minimal filesystem permissions required to read/write within the workspace.
3. **Environment Isolation**: Never run the MCP HTTP server on a shared production server. It is strictly a local development and staging orchestration utility.
4. **Token Security**: Treat `BFFX_MCP_TOKEN` as a high-security credential. Never commit it to git repositories or share it in cleartext.

