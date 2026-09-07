package hooks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

func noteOwnerID(note map[string]any) string {
	if note == nil {
		return ""
	}
	return fmt.Sprintf("%v", note["created_by"])
}

const maxNoteTitleLen = 200

func BeforeCreateNote(ctx *handlers.ActionContext, payload map[string]any) error {
	title, _ := payload["title"].(string)
	if len(strings.TrimSpace(title)) == 0 {
		return fmt.Errorf("title is required")
	}
	if len(title) > maxNoteTitleLen {
		return fmt.Errorf("title must be at most %d characters", maxNoteTitleLen)
	}
	return nil
}

func AfterCreateNote(ctx *handlers.ActionContext, result map[string]any) error {
	id, _ := result["id"].(string)
	ctx.Analytics.Track(ctx.Context, "note_created", map[string]any{"note_id": id})
	title, _ := result["title"].(string)
	if uid := ctx.UserID(); uid != "" {
		_ = ctx.Notify.NotifyUser(ctx.Context, uid, "Note saved", title, map[string]string{"note_id": id})
	}
	return nil
}

// HandleShareNote notifies recipient users that a note was shared with them.
func HandleShareNote(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	var body struct {
		NoteID           string   `json:"note_id"`
		RecipientUserIDs []string `json:"recipient_user_ids"`
		Title            string   `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errors.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if body.NoteID == "" || len(body.RecipientUserIDs) == 0 {
		errors.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "note_id and recipient_user_ids required"})
		return
	}
	note, err := ctx.Store.Get(r.Context(), "Note", body.NoteID)
	if err != nil || note == nil {
		errors.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
		return
	}
	owner := noteOwnerID(note)
	cur := ctx.UserID()
	if cur == "" || owner != cur {
		errors.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	msg := body.Title
	if msg == "" {
		msg, _ = note["title"].(string)
	}
	for _, uid := range body.RecipientUserIDs {
		if uid == "" {
			continue
		}
		_ = ctx.Notify.NotifyUser(ctx.Context, uid, "Note shared with you", msg, map[string]string{"note_id": body.NoteID})
	}
	ctx.Analytics.Track(ctx.Context, "note_shared", map[string]any{"note_id": body.NoteID, "recipients": len(body.RecipientUserIDs)})
	errors.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "delivered_to": len(body.RecipientUserIDs)})
}
