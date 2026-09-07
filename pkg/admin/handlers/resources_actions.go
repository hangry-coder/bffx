package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hangry-coder/bffx/pkg/admin/hooks"
	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func (h *ResourceHandler) HandleBatchAction(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "write"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	actionName := r.PathValue("action")

	var reqBody struct {
		IDs     []string       `json:"ids"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	graph, err := manifest.BuildAdminGraph(h.reg)
	if err != nil || graph == nil {
		http.Error(w, "failed to build admin graph", http.StatusInternalServerError)
		return
	}

	var actionSpec *manifest.AdminResourceAction
	for _, res := range graph.Resources {
		if strings.EqualFold(res.Resource, resource) {
			for _, act := range res.BatchActions {
				if strings.EqualFold(act.Name, actionName) {
					actionSpec = &act
					break
				}
			}
			break
		}
	}

	if actionSpec == nil {
		http.Error(w, "action not found", http.StatusNotFound)
		return
	}

	if actionSpec.Hook == "" {
		http.Error(w, "hook not configured for action", http.StatusBadRequest)
		return
	}

	hookFn, ok := hooks.Get(actionSpec.Hook)
	if !ok {
		http.Error(w, fmt.Sprintf("hook %q not registered", actionSpec.Hook), http.StatusInternalServerError)
		return
	}

	actorID, _ := r.Context().Value("admin_id").(string)
	ctx := &hooks.Context{
		Context: r.Context(),
		Store:   h.store,
		Auditor: h.auditor,
		ActorID: actorID,
	}

	result, err := hookFn(ctx, resource, reqBody.IDs, reqBody.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit Logging
	if h.auditor != nil {
		payloadStr, _ := json.Marshal(map[string]any{
			"ids":     reqBody.IDs,
			"payload": reqBody.Payload,
			"result":  result,
		})
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "batch_action:" + actionName,
			ResourceKind: resource,
			Payload:      string(payloadStr),
			IPAddress:    r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if result == nil {
		result = map[string]any{"status": "success"}
	}
	json.NewEncoder(w).Encode(result)
}

func (h *ResourceHandler) HandleMemberAction(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "write"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	actionName := r.PathValue("action")

	var payload map[string]any
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	graph, err := manifest.BuildAdminGraph(h.reg)
	if err != nil || graph == nil {
		http.Error(w, "failed to build admin graph", http.StatusInternalServerError)
		return
	}

	var actionSpec *manifest.AdminResourceAction
	for _, res := range graph.Resources {
		if strings.EqualFold(res.Resource, resource) {
			for _, act := range res.MemberActions {
				if strings.EqualFold(act.Name, actionName) {
					actionSpec = &act
					break
				}
			}
			break
		}
	}

	if actionSpec == nil {
		http.Error(w, "action not found", http.StatusNotFound)
		return
	}

	if actionSpec.Hook == "" {
		http.Error(w, "hook not configured for action", http.StatusBadRequest)
		return
	}

	hookFn, ok := hooks.Get(actionSpec.Hook)
	if !ok {
		http.Error(w, fmt.Sprintf("hook %q not registered", actionSpec.Hook), http.StatusInternalServerError)
		return
	}

	actorID, _ := r.Context().Value("admin_id").(string)
	ctx := &hooks.Context{
		Context: r.Context(),
		Store:   h.store,
		Auditor: h.auditor,
		ActorID: actorID,
	}

	result, err := hookFn(ctx, resource, []string{id}, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit Logging
	if h.auditor != nil {
		payloadStr, _ := json.Marshal(map[string]any{
			"payload": payload,
			"result":  result,
		})
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "member_action:" + actionName,
			ResourceKind: resource,
			ResourceID:   id,
			Payload:      string(payloadStr),
			IPAddress:    r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if result == nil {
		result = map[string]any{"status": "success"}
	}
	json.NewEncoder(w).Encode(result)
}

func (h *ResourceHandler) HandleCollectionAction(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "write"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	actionName := r.PathValue("action")

	var payload map[string]any
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	graph, err := manifest.BuildAdminGraph(h.reg)
	if err != nil || graph == nil {
		http.Error(w, "failed to build admin graph", http.StatusInternalServerError)
		return
	}

	var actionSpec *manifest.AdminResourceAction
	for _, res := range graph.Resources {
		if strings.EqualFold(res.Resource, resource) {
			for _, act := range res.CollectionActions {
				if strings.EqualFold(act.Name, actionName) {
					actionSpec = &act
					break
				}
			}
			break
		}
	}

	if actionSpec == nil {
		http.Error(w, "action not found", http.StatusNotFound)
		return
	}

	if actionSpec.Hook == "" {
		http.Error(w, "hook not configured for action", http.StatusBadRequest)
		return
	}

	hookFn, ok := hooks.Get(actionSpec.Hook)
	if !ok {
		http.Error(w, fmt.Sprintf("hook %q not registered", actionSpec.Hook), http.StatusInternalServerError)
		return
	}

	actorID, _ := r.Context().Value("admin_id").(string)
	ctx := &hooks.Context{
		Context: r.Context(),
		Store:   h.store,
		Auditor: h.auditor,
		ActorID: actorID,
	}

	result, err := hookFn(ctx, resource, nil, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit Logging
	if h.auditor != nil {
		payloadStr, _ := json.Marshal(map[string]any{
			"payload": payload,
			"result":  result,
		})
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "collection_action:" + actionName,
			ResourceKind: resource,
			Payload:      string(payloadStr),
			IPAddress:    r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if result == nil {
		result = map[string]any{"status": "success"}
	}
	json.NewEncoder(w).Encode(result)
}
