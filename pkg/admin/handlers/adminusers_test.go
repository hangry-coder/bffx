package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	bffx_errors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestDeleteAdminUser(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminUserHandler(store, nil)
	ctx := context.Background()

	// Seed one operator and two superusers
	op, _ := store.Create(ctx, "AdminUser", map[string]any{"email": "op@test.com", "role": "operator"})
	su1, _ := store.Create(ctx, "AdminUser", map[string]any{"email": "su1@test.com", "role": "Superuser"})
	su2, _ := store.Create(ctx, "AdminUser", map[string]any{"email": "su2@test.com", "role": "Superuser"})

	// 1. Delete operator (should work)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/AdminUser/"+op["id"].(string), nil)
	req.SetPathValue("id", op["id"].(string))
	w := httptest.NewRecorder()
	h.DeleteAdminUser(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify operator is deleted
	val, err := store.Get(ctx, "AdminUser", op["id"].(string))
	assert.ErrorIs(t, err, bffx_errors.ErrNotFound)
	assert.Nil(t, val)

	// 2. Delete su1 (should work since su2 is still there)
	req = httptest.NewRequest(http.MethodDelete, "/api/admin/resources/AdminUser/"+su1["id"].(string), nil)
	req.SetPathValue("id", su1["id"].(string))
	w = httptest.NewRecorder()
	h.DeleteAdminUser(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify su1 is deleted
	val, err = store.Get(ctx, "AdminUser", su1["id"].(string))
	assert.ErrorIs(t, err, bffx_errors.ErrNotFound)
	assert.Nil(t, val)

	// 3. Delete su2 (should fail as it is the last superuser)
	req = httptest.NewRequest(http.MethodDelete, "/api/admin/resources/AdminUser/"+su2["id"].(string), nil)
	req.SetPathValue("id", su2["id"].(string))
	w = httptest.NewRecorder()
	h.DeleteAdminUser(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "Cannot delete the last superuser", errResp["error"])

	// Verify su2 is NOT deleted
	val, err = store.Get(ctx, "AdminUser", su2["id"].(string))
	assert.NoError(t, err)
	assert.NotNil(t, val)
}
