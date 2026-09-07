package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
)

func TestAsyncIngestionPipeline_JobQueueing(t *testing.T) {
	s := storage.NewMemoryStore()
	bus := events.NewMemoryBus()
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(s, nil)
	jwt := auth.NewJWTService("test-secret")

	// Construct async ingestion pipeline manifest
	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: Scanner
spec:
  type: ingestion
  execution: async
  route:
    method: POST
    path: /api/v1/scanner
    auth: optional
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	err := yaml.Unmarshal([]byte(pipelineYaml), &m)
	require.NoError(t, err)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Pipelines: []*manifest.Manifest{m},
	}

	jobStore := worker.NewMemoryJobStore()
	queue := worker.NewMemoryQueue()

	r := NewRouter(RouterConfig{
		Store:        s,
		Telemetry:    s,
		JobStore:     jobStore,
		Queue:        queue,
		Registry:     reg,
		AuthProvider: jwt,
		JWTService:   jwt,
		EventBus:     bus,
		I18n:         bundle,
		FlagProvider: flagProvider,
	})

	handler := r.Setup()

	// 1. Send request to async ingestion endpoint
	reqBody := "raw-image-bytes-mock-data"
	req := httptest.NewRequest("POST", "/api/v1/scanner", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/octet-stream")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	// 2. Decode status and job ID
	var resp struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.JobID)
	assert.Equal(t, "pending", resp.Status)

	// 3. Verify job was enqueued in job store
	job, err := jobStore.Get(context.Background(), resp.JobID)
	require.NoError(t, err)
	assert.Equal(t, "pipeline.ingestion", job.Kind)
	assert.Equal(t, "Scanner", job.Name)
	assert.Equal(t, "pending", job.Status)
	assert.Equal(t, "raw-image-bytes-mock-data", job.Input["media"])

	// 4. Verify job was pushed into the queue
	poppedJob, err := queue.Pop(context.Background())
	require.NoError(t, err)
	assert.Equal(t, job.ID, poppedJob.ID)
	assert.Equal(t, "pipeline.ingestion", poppedJob.Kind)
}
