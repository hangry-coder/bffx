package blob

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

// LocalProvider serves dev-friendly presigned PUT/GET against this process
// (see UploadsHandler.LocalPut / LocalGet). baseURL in Presign* must be the public origin (scheme+host).
type LocalProvider struct {
	secret []byte
	dir    string
}

func NewLocalProvider(secret string, dir string) *LocalProvider {
	return &LocalProvider{secret: []byte(secret), dir: dir}
}

func (l *LocalProvider) sign(op, objectKey string, expUnix int64) string {
	mac := hmac.New(sha256.New, l.secret)
	_, _ = fmt.Fprintf(mac, "%s|%s|%d", op, objectKey, expUnix)
	return hex.EncodeToString(mac.Sum(nil))
}

func (l *LocalProvider) PresignPut(ctx context.Context, baseURL, objectKey string, ttl time.Duration) (PresignResult, error) {
	return l.presign(ctx, baseURL, "PUT", objectKey, ttl)
}

func (l *LocalProvider) PresignGet(ctx context.Context, baseURL, objectKey string, ttl time.Duration) (PresignResult, error) {
	return l.presign(ctx, baseURL, "GET", objectKey, ttl)
}

func (l *LocalProvider) presign(_ context.Context, baseURL, op, objectKey string, ttl time.Duration) (PresignResult, error) {
	if err := ValidateObjectKey(objectKey); err != nil {
		return PresignResult{}, err
	}
	exp := time.Now().Add(ttl).UTC()
	expUnix := exp.Unix()
	sig := l.sign(op, objectKey, expUnix)
	raw, err := url.JoinPath(strings.TrimRight(baseURL, "/"), "api", "v1", "uploads", "local")
	if err != nil {
		return PresignResult{}, err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return PresignResult{}, err
	}
	q := u.Query()
	q.Set("k", objectKey)
	q.Set("e", strconv.FormatInt(expUnix, 10))
	q.Set("m", op)
	q.Set("s", sig)
	u.RawQuery = q.Encode()
	return PresignResult{
		URL:       u.String(),
		Method:    op,
		ExpiresAt: exp,
	}, nil
}

// PublicURL returns a presigned GET URL with a long TTL (24h) for local storage.
func (l *LocalProvider) PublicURL(baseURL, objectKey string) string {
	res, err := l.PresignGet(context.Background(), baseURL, objectKey, 24*time.Hour)
	if err != nil {
		return ""
	}
	return res.URL
}

// Delete removes the file from the local filesystem.
func (l *LocalProvider) Delete(_ context.Context, objectKey string) error {
	path, err := SafeJoinObjectPath(l.dir, objectKey)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("blob: delete local: %w", err)
	}
	return nil
}

// Type returns "local".
func (l *LocalProvider) Type() string {
	return "local"
}

// Ping checks if the local upload directory is writable.
func (l *LocalProvider) Ping(_ context.Context) error {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("blob: ping local (mkdir): %w", err)
	}
	// Try creating a temporary file
	f, err := os.CreateTemp(l.dir, ".bffx-ping-*")
	if err != nil {
		return fmt.Errorf("blob: ping local (create): %w", err)
	}
	f.Close()
	os.Remove(f.Name())
	return nil
}

// VerifyLocalSignature checks HMAC for a local presigned URL query (k, e, m, s).
func VerifyLocalSignature(secret []byte, objectKey string, expUnix int64, op, sigHex string) bool {
	mac := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(mac, "%s|%s|%d", op, objectKey, expUnix)
	want := mac.Sum(nil)
	got, err := hex.DecodeString(strings.TrimSpace(sigHex))
	if err != nil || len(got) != len(want) {
		return false
	}
	return hmac.Equal(got, want)
}

// SafeJoinObjectPath resolves objectKey under root for filesystem writes.
func SafeJoinObjectPath(root, objectKey string) (string, error) {
	if err := ValidateObjectKey(objectKey); err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel := filepath.Clean(filepath.FromSlash(strings.TrimLeft(objectKey, "/")))
	full := filepath.Join(rootAbs, rel)
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	sep := string(os.PathSeparator)
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs+sep, rootAbs+sep) {
		return "", fmt.Errorf("blob: object key escapes root")
	}
	return fullAbs, nil
}

// ValidateUploadManifestField checks resource manifest for bucket: uploads on the named field.
func ValidateUploadManifestField(reg *manifest.Registry, resourceName, fieldName string) error {
	if resourceName == "" && fieldName == "" {
		return nil
	}
	if resourceName == "" || fieldName == "" {
		return fmt.Errorf("blob: resource and field are both required for manifest validation")
	}
	m, ok := reg.GetResource(resourceName)
	if !ok {
		return fmt.Errorf("blob: unknown resource %q", resourceName)
	}
	var spec manifest.ResourceSpec
	if err := m.UnmarshalSpec(&spec); err != nil {
		return err
	}
	for _, f := range spec.Fields {
		if f.Name == fieldName {
			if strings.TrimSpace(f.Bucket) == "uploads" {
				return nil
			}
			return fmt.Errorf("blob: field %q does not declare bucket: uploads", fieldName)
		}
	}
	return fmt.Errorf("blob: field %q not found on resource %q", fieldName, resourceName)
}

func localPutHTTP(secret []byte, rootDir string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	key := q.Get("k")
	expStr := q.Get("e")
	sig := q.Get("s")
	op := q.Get("m")
	if op == "" {
		op = "PUT"
	}
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		http.Error(w, "expired or invalid", http.StatusForbidden)
		return
	}
	if op != "PUT" || !VerifyLocalSignature(secret, key, expUnix, op, sig) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}
	path, err := SafeJoinObjectPath(rootDir, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		http.Error(w, "mkdir", http.StatusInternalServerError)
		return
	}
	f, err := os.Create(path)
	if err != nil {
		http.Error(w, "create", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(r.Body, 32<<20)); err != nil {
		http.Error(w, "write", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func localGetHTTP(secret []byte, rootDir string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	key := q.Get("k")
	expStr := q.Get("e")
	sig := q.Get("s")
	op := q.Get("m")
	if op == "" {
		op = "GET"
	}
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		http.Error(w, "expired or invalid", http.StatusForbidden)
		return
	}
	if op != "GET" || !VerifyLocalSignature(secret, key, expUnix, op, sig) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}
	path, err := SafeJoinObjectPath(rootDir, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, path)
}

// ServePut handles PUT /api/v1/uploads/local for URLs issued by this provider.
func (l *LocalProvider) ServePut(w http.ResponseWriter, r *http.Request) {
	localPutHTTP(l.secret, l.dir, w, r)
}

// ServeGet handles GET /api/v1/uploads/local for URLs issued by this provider.
func (l *LocalProvider) ServeGet(w http.ResponseWriter, r *http.Request) {
	localGetHTTP(l.secret, l.dir, w, r)
}
