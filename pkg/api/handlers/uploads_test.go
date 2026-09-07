package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage/blob"

	"gopkg.in/yaml.v3"
)

func TestUploadsHandler_Presign_Disabled(t *testing.T) {
	h := NewUploadsHandler(nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presign", bytes.NewBufferString(`{"key":"a/b"}`))
	h.Presign(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestUploadsHandler_Presign_LocalPut(t *testing.T) {
	dir := t.TempDir()
	lp := blob.NewLocalProvider("secret-for-upload-tests-32b", dir)
	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "T"},
			Spec:     yaml.Node{},
		},
	}
	var docSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
fields:
  - {name: pic, type: string, bucket: uploads}
`), &docSpec); err != nil {
		t.Fatal(err)
	}
	reg.Resources = []*manifest.Manifest{{Metadata: manifest.Metadata{Name: "Doc"}, Spec: docSpec}}

	h := NewUploadsHandler(lp, reg)
	body := map[string]any{
		"key":      "u/1/f.bin",
		"resource": "Doc",
		"field":    "pic",
		"op":       "put",
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presign", bytes.NewReader(b))
	req.Host = "127.0.0.1:9"
	h.Presign(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	putURL, _ := out["url"].(string)
	if putURL == "" {
		t.Fatal("missing url")
	}

	putReq, err := http.NewRequestWithContext(context.Background(), http.MethodPut, putURL, bytes.NewBufferString("payload"))
	if err != nil {
		t.Fatal(err)
	}
	putRec := httptest.NewRecorder()
	lp.ServePut(putRec, putReq)
	if putRec.Code != http.StatusNoContent {
		t.Fatalf("put %d %s", putRec.Code, putRec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(dir, "u", "1", "f.bin"))
	if err != nil || string(got) != "payload" {
		t.Fatalf("file err=%v data=%q", err, got)
	}
}

func TestUploadsHandler_Presign_InvalidKey(t *testing.T) {
	lp := blob.NewLocalProvider("secret-for-upload-tests-32b", t.TempDir())
	h := NewUploadsHandler(lp, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presign", bytes.NewBufferString(`{"key":"../x","op":"put"}`))
	req.Host = "localhost"
	h.Presign(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestUploadsHandler_Presign_ManifestMismatch(t *testing.T) {
	lp := blob.NewLocalProvider("secret-for-upload-tests-32b", t.TempDir())
	var docSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
fields:
  - {name: pic, type: string}
`), &docSpec); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{
		Project:   &manifest.Manifest{Metadata: manifest.Metadata{Name: "T"}, Spec: yaml.Node{}},
		Resources: []*manifest.Manifest{{Metadata: manifest.Metadata{Name: "Doc"}, Spec: docSpec}},
	}
	h := NewUploadsHandler(lp, reg)
	b, _ := json.Marshal(map[string]any{"key": "k", "resource": "Doc", "field": "pic", "op": "put"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presign", bytes.NewReader(b))
	req.Host = "localhost"
	h.Presign(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d %s", rec.Code, rec.Body.String())
	}
}

func TestUploadsHandler_Presign_RequiresBothResourceAndField(t *testing.T) {
	lp := blob.NewLocalProvider("secret-for-upload-tests-32b", t.TempDir())
	h := NewUploadsHandler(lp, &manifest.Registry{})
	b, _ := json.Marshal(map[string]any{"key": "k", "resource": "Doc", "op": "put"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presign", bytes.NewReader(b))
	req.Host = "localhost"
	h.Presign(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestUploadsHandler_Presign_GetOp(t *testing.T) {
	dir := t.TempDir()
	lp := blob.NewLocalProvider("secret-for-upload-tests-32b", dir)
	path := filepath.Join(dir, "a", "b.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewUploadsHandler(lp, nil)
	b, _ := json.Marshal(map[string]any{"key": "a/b.txt", "op": "get"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presign", bytes.NewReader(b))
	req.Host = "localhost:1"
	h.Presign(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	getURL, _ := out["url"].(string)
	greq := httptest.NewRequest(http.MethodGet, getURL, nil)
	grec := httptest.NewRecorder()
	lp.ServeGet(grec, greq)
	if grec.Code != http.StatusOK {
		t.Fatalf("get %d", grec.Code)
	}
	if grec.Body.String() != "z" {
		t.Fatalf("body %q", grec.Body.String())
	}
}
