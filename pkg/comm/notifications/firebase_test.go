package notifications

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFCM_MissingCredentials_ReturnsError(t *testing.T) {
	p := NewFirebaseV1Provider("", nil)
	err := p.Send(context.Background(), "device-token", "t", "b", nil)
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("missing creds: got %v want %v", err, ErrMissingCredentials)
	}
}

func mustFakeServiceAccountJSON(t *testing.T, projectID string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	var pemBuf bytes.Buffer
	if err := pem.Encode(&pemBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   projectID,
		"private_key":  pemBuf.String(),
		"client_email": "bffx-test@" + projectID + ".iam.gserviceaccount.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFCM_200_HappyPath(t *testing.T) {
	sa := mustFakeServiceAccountJSON(t, "proj-happy")
	var sawFCMBody atomic.Bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token" && r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access","token_type":"Bearer"}`))
		case strings.HasSuffix(r.URL.Path, "/messages:send"):
			b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			_ = r.Body.Close()
			if !bytes.Contains(b, []byte(`"token":"devtok"`)) {
				t.Errorf("FCM body missing token: %s", b)
			}
			sawFCMBody.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	p := NewFirebaseV1Provider("", sa)
	p.OAuthTokenURL = ts.URL + "/token"
	p.FCMSendURL = ts.URL + "/v1/projects/proj-happy/messages:send"
	p.HTTPClient = ts.Client()

	if err := p.Send(context.Background(), "devtok", "Hello", "World", map[string]string{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if !sawFCMBody.Load() {
		t.Fatal("expected FCM handler to run")
	}
}

func TestFCM_401_RetriesOAuthOnce(t *testing.T) {
	sa := mustFakeServiceAccountJSON(t, "proj-401")
	var tokenPosts atomic.Int32
	var fcmPosts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token" && r.Method == http.MethodPost:
			tokenPosts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access","token_type":"Bearer"}`))
		case strings.HasSuffix(r.URL.Path, "/messages:send"):
			n := fcmPosts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	p := NewFirebaseV1Provider("", sa)
	p.OAuthTokenURL = ts.URL + "/token"
	p.FCMSendURL = ts.URL + "/v1/projects/proj-401/messages:send"
	p.HTTPClient = ts.Client()

	if err := p.Send(context.Background(), "devtok", "t", "b", nil); err != nil {
		t.Fatal(err)
	}
	if tokenPosts.Load() < 2 {
		t.Fatalf("expected at least 2 OAuth token exchanges, got %d", tokenPosts.Load())
	}
	if fcmPosts.Load() != 2 {
		t.Fatalf("expected 2 FCM posts, got %d", fcmPosts.Load())
	}
}

func TestFCM_404_InvalidDeviceToken(t *testing.T) {
	sa := mustFakeServiceAccountJSON(t, "proj-404")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token" && r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access","token_type":"Bearer"}`))
		case strings.HasSuffix(r.URL.Path, "/messages:send"):
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	p := NewFirebaseV1Provider("", sa)
	p.OAuthTokenURL = ts.URL + "/token"
	p.FCMSendURL = ts.URL + "/v1/projects/proj-404/messages:send"
	p.HTTPClient = ts.Client()

	err := p.Send(context.Background(), "badtok", "t", "b", nil)
	if !errors.Is(err, ErrInvalidDeviceToken) {
		t.Fatalf("got %v want %v", err, ErrInvalidDeviceToken)
	}
}

func TestFCM_429_RetryThenOK(t *testing.T) {
	sa := mustFakeServiceAccountJSON(t, "proj-429")
	var fcmPosts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token" && r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access","token_type":"Bearer"}`))
		case strings.HasSuffix(r.URL.Path, "/messages:send"):
			n := fcmPosts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			if n == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	p := NewFirebaseV1Provider("", sa)
	p.OAuthTokenURL = ts.URL + "/token"
	p.FCMSendURL = ts.URL + "/v1/projects/proj-429/messages:send"
	p.HTTPClient = ts.Client()

	start := time.Now()
	if err := p.Send(context.Background(), "devtok", "t", "b", nil); err != nil {
		t.Fatal(err)
	}
	if fcmPosts.Load() != 2 {
		t.Fatalf("expected 2 FCM posts, got %d", fcmPosts.Load())
	}
	if time.Since(start) < 70*time.Millisecond {
		t.Fatalf("expected backoff after 429, elapsed=%s", time.Since(start))
	}
}
