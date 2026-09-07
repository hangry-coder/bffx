package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/worker"

	"github.com/stretchr/testify/assert"
)

func TestJobsHandler_Dispatch_Get_List_WriteBack(t *testing.T) {
	store := worker.NewMemoryJobStore()
	h := NewJobsHandler(store, nil)

	ctx := middleware.WithClaims(context.Background(), map[string]any{"sub": "user-1"})

	t.Run("Dispatch_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/jobs/dispatch", bytes.NewBufferString("not-json"))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.Dispatch(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Dispatch_InvalidKind", func(t *testing.T) {
		body := `{"kind":"bad","name":"n","input":{}}`
		req := httptest.NewRequest("POST", "/jobs/dispatch", bytes.NewBufferString(body))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.Dispatch(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Dispatch_EmptyName", func(t *testing.T) {
		body := `{"kind":"skill","name":"","input":{}}`
		req := httptest.NewRequest("POST", "/jobs/dispatch", bytes.NewBufferString(body))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.Dispatch(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Dispatch_Success", func(t *testing.T) {
		body := `{"kind":"skill","name":"doit","input":{"x":1}}`
		req := httptest.NewRequest("POST", "/jobs/dispatch", bytes.NewBufferString(body))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.Dispatch(rr, req)
		assert.Equal(t, http.StatusCreated, rr.Code)
		var job worker.Job
		assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &job))
		assert.NotEmpty(t, job.ID)
		assert.Equal(t, "user-1", job.UserID)

		t.Run("GetJob", func(t *testing.T) {
			req2 := httptest.NewRequest("GET", "/jobs/"+job.ID, nil)
			req2.SetPathValue("id", job.ID)
			req2 = req2.WithContext(ctx)
			rr2 := httptest.NewRecorder()
			h.GetJob(rr2, req2)
			assert.Equal(t, http.StatusOK, rr2.Code)
		})

		t.Run("GetJob_NotFound", func(t *testing.T) {
			req2 := httptest.NewRequest("GET", "/jobs/nope", nil)
			req2.SetPathValue("id", "nope")
			req2 = req2.WithContext(ctx)
			rr2 := httptest.NewRecorder()
			h.GetJob(rr2, req2)
			assert.Equal(t, http.StatusNotFound, rr2.Code)
		})

		t.Run("GetJob_ForbiddenOtherUser", func(t *testing.T) {
			ctx2 := middleware.WithClaims(context.Background(), map[string]any{"sub": "user-2"})
			req2 := httptest.NewRequest("GET", "/jobs/"+job.ID, nil)
			req2.SetPathValue("id", job.ID)
			req2 = req2.WithContext(ctx2)
			rr2 := httptest.NewRecorder()
			h.GetJob(rr2, req2)
			assert.Equal(t, http.StatusForbidden, rr2.Code)
		})

		t.Run("WriteBack", func(t *testing.T) {
			patch := `{"status":"done","result":{"ok":true},"error":""}`
			req2 := httptest.NewRequest("PATCH", "/jobs/"+job.ID+"/result", bytes.NewBufferString(patch))
			req2.SetPathValue("id", job.ID)
			req2 = req2.WithContext(ctx)
			rr2 := httptest.NewRecorder()
			h.WriteBack(rr2, req2)
			assert.Equal(t, http.StatusOK, rr2.Code)
		})

		t.Run("WriteBack_InvalidStatus", func(t *testing.T) {
			patch := `{"status":"pending","result":null,"error":""}`
			req2 := httptest.NewRequest("PATCH", "/jobs/"+job.ID+"/result", bytes.NewBufferString(patch))
			req2.SetPathValue("id", job.ID)
			req2 = req2.WithContext(ctx)
			rr2 := httptest.NewRecorder()
			h.WriteBack(rr2, req2)
			assert.Equal(t, http.StatusBadRequest, rr2.Code)
		})
	})

	t.Run("ListJobs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/jobs", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.ListJobs(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		var out struct {
			Items []worker.Job `json:"items"`
		}
		assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		assert.GreaterOrEqual(t, len(out.Items), 1)
	})
}
