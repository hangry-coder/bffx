package notifications

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Sentinel errors for FCM HTTP v1.
var (
	ErrMissingCredentials = errors.New("fcm: missing service account credentials")
	ErrInvalidDeviceToken = errors.New("fcm: invalid or unregistered device token")
	ErrFCMUnavailable     = errors.New("fcm: upstream temporarily unavailable")
)

type serviceAccountFile struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
}

// FirebaseV1Provider sends push notifications via FCM HTTP v1 + OAuth2
// (service account JWT bearer). Set OAuthTokenURL / FCMSendURL for tests only.
type FirebaseV1Provider struct {
	ProjectID      string
	ServiceAccount []byte

	// Optional overrides (tests); production leaves zero.
	OAuthTokenURL string
	FCMSendURL    string

	HTTPClient *http.Client
}

func NewFirebaseV1Provider(projectID string, credentials []byte) *FirebaseV1Provider {
	return &FirebaseV1Provider{
		ProjectID:      projectID,
		ServiceAccount: credentials,
	}
}

func (f *FirebaseV1Provider) httpClient() *http.Client {
	if f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}

func (f *FirebaseV1Provider) oauthTokenURL() string {
	if f.OAuthTokenURL != "" {
		return f.OAuthTokenURL
	}
	return "https://oauth2.googleapis.com/token"
}

func (f *FirebaseV1Provider) fcmSendURL(projectID string) string {
	if f.FCMSendURL != "" {
		return f.FCMSendURL
	}
	return "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(projectID) + "/messages:send"
}

func parseServiceAccountJSON(raw []byte) (*serviceAccountFile, *rsa.PrivateKey, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil, ErrMissingCredentials
	}
	var sa serviceAccountFile
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, nil, fmt.Errorf("fcm: parse service account json: %w", err)
	}
	if sa.Type != "" && sa.Type != "service_account" {
		return nil, nil, fmt.Errorf("fcm: unexpected service account type %q", sa.Type)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, nil, fmt.Errorf("fcm: service account missing client_email or private_key")
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, nil, fmt.Errorf("fcm: private_key is not valid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("fcm: private key is not RSA")
		}
		return &sa, rsaKey, nil
	}
	pkcs1, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err2 != nil {
		return nil, nil, fmt.Errorf("fcm: parse private key: %w", err)
	}
	return &sa, pkcs1, nil
}

func (f *FirebaseV1Provider) mintJWTAssertion(sa *serviceAccountFile, key *rsa.PrivateKey) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":   sa.ClientEmail,
		"sub":   sa.ClientEmail,
		"aud":   "https://oauth2.googleapis.com/token",
		"iat":   now.Unix(),
		"exp":   now.Add(55 * time.Minute).Unix(),
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(key)
}

func (f *FirebaseV1Provider) fetchAccessToken(ctx context.Context, assertion string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.oauthTokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fcm: oauth token %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("fcm: oauth response: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("fcm: oauth missing access_token")
	}
	return out.AccessToken, nil
}

func fcmMessagePayload(token, title, body string, data map[string]string) ([]byte, error) {
	msg := map[string]any{
		"message": map[string]any{
			"token": token,
			"notification": map[string]string{
				"title": title,
				"body":  body,
			},
		},
	}
	if len(data) > 0 {
		msg["message"].(map[string]any)["data"] = data
	}
	return json.Marshal(msg)
}

func (f *FirebaseV1Provider) postFCM(ctx context.Context, accessToken, projectID, token, title, body string, data map[string]string) (int, []byte, http.Header, error) {
	payload, err := fcmMessagePayload(token, title, body, data)
	if err != nil {
		return 0, nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.fcmSendURL(projectID), bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := f.httpClient().Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, b, resp.Header.Clone(), nil
}

func retryAfterFromHeader(h http.Header) time.Duration {
	s := strings.TrimSpace(h.Get("Retry-After"))
	if s == "" {
		return 0
	}
	if sec, err := strconv.Atoi(s); err == nil {
		return time.Duration(sec) * time.Second
	}
	return 0
}

// Send delivers a notification via FCM HTTP v1. ctx is honored on all HTTP calls.
func (f *FirebaseV1Provider) Send(ctx context.Context, token string, title, body string, data map[string]string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("fcm: empty device token")
	}
	sa, key, err := parseServiceAccountJSON(f.ServiceAccount)
	if err != nil {
		if errors.Is(err, ErrMissingCredentials) {
			return err
		}
		return fmt.Errorf("fcm: %w", err)
	}
	projectID := strings.TrimSpace(f.ProjectID)
	if projectID == "" {
		projectID = strings.TrimSpace(sa.ProjectID)
	}
	if projectID == "" {
		return fmt.Errorf("fcm: project id missing (set ProjectID or project_id in JSON)")
	}

	assertion, err := f.mintJWTAssertion(sa, key)
	if err != nil {
		return fmt.Errorf("fcm: mint jwt: %w", err)
	}

	access, err := f.fetchAccessToken(ctx, assertion)
	if err != nil {
		return err
	}

	const maxAttempts = 4
	backoff := 80 * time.Millisecond

	var lastStatus int
	var lastBody []byte

	sendOnce := func(accessTok string) (int, []byte, http.Header, error) {
		return f.postFCM(ctx, accessTok, projectID, token, title, body, data)
	}

	status, respBody, hdr, err := sendOnce(access)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		assertion, err = f.mintJWTAssertion(sa, key)
		if err != nil {
			return err
		}
		access, err = f.fetchAccessToken(ctx, assertion)
		if err != nil {
			return err
		}
		status, respBody, hdr, err = sendOnce(access)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastStatus, lastBody = status, respBody

		switch status {
		case http.StatusOK:
			return nil
		case http.StatusNotFound:
			return fmt.Errorf("%w", ErrInvalidDeviceToken)
		case http.StatusTooManyRequests:
			if attempt == maxAttempts-1 {
				return fmt.Errorf("%w: %s", ErrFCMUnavailable, strings.TrimSpace(string(respBody)))
			}
			d := backoff
			if ra := retryAfterFromHeader(hdr); ra > 0 {
				d = ra
			}
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
			status, respBody, hdr, err = sendOnce(access)
			if err != nil {
				return err
			}
			continue
		default:
			if status >= 500 && status < 600 && attempt < maxAttempts-1 {
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return ctx.Err()
				}
				backoff *= 2
				status, respBody, hdr, err = sendOnce(access)
				if err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("fcm: send %d: %s", status, strings.TrimSpace(string(respBody)))
		}
	}
	return fmt.Errorf("fcm: send exhausted retries (last %d: %s)", lastStatus, strings.TrimSpace(string(lastBody)))
}
