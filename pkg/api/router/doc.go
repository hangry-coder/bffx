// Package router registers HTTP routes from the compiled manifest graph (CRUD,
// screens, builders, actions, streams, auth, jobs, billing stubs).
//
// Extension points (where to edit):
//   - New route families (e.g. feeds): add a register* method and call it from Setup in router.go.
//   - CRUD + list pagination: crud.go.
//   - Policy gates on routes: policy_wrap.go.
//   - Manifest hook execution: hooks_execute.go.
//   - Action routes + per-route auth wrapper: actions.go.
//   - Screen / builder payloads: screens_builders.go.
//   - SSE streams: streams.go.
//   - JSON helpers and dot-path resolution for screen output: helpers_http.go.
//   - URL path helpers (collection paths, owner policy): paths.go.
package router
