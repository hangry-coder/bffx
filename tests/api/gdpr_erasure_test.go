package api_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

// TestGDPRUserErasure_CascadingDeletion verifies that the cascading SQL deletion
// patterns specified in our GDPR right to erasure compliance playbook physically
// erase the user, clear their session token history, and dissociate their device mapping.
func TestGDPRUserErasure_CascadingDeletion(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "gdpr.db")
	st, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.GetDB().Close()

	// 1. Setup core User, RefreshToken, and Device manifests
	var userSpec, refreshSpec, deviceSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`fields:
  - {name: email, type: string, required: true, unique: true}
  - {name: password, type: string, required: true}
  - {name: name, type: string}
  - {name: role, type: string}
`), &userSpec); err != nil {
		t.Fatal(err)
	}

	if err := yaml.Unmarshal([]byte(`fields:
  - {name: user_id, type: string, target: User}
  - {name: token, type: string, unique: true}
`), &refreshSpec); err != nil {
		t.Fatal(err)
	}

	if err := yaml.Unmarshal([]byte(`fields:
  - {name: user_id, type: string, target: User}
  - {name: device_uuid, type: string, unique: true}
`), &deviceSpec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "GDPRTest"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "RefreshToken"}, Spec: refreshSpec},
			{Metadata: manifest.Metadata{Name: "Device"}, Spec: deviceSpec},
		},
	}

	// 2. Run Database Schema Auto-Reconciliation
	prevWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if _, err := st.Reconcile(context.Background(), reg); err != nil {
		os.Chdir(prevWD)
		t.Fatalf("reconcile: %v", err)
	}
	os.Chdir(prevWD)

	db := st.GetDB()

	// 3. Seed Fixture Data
	userID := "usr_gdpr_123"
	emailAddr := "gdpr-subject@domain.com"
	
	// Seed core User
	_, err = db.Exec(`INSERT INTO "bffx_user" (id, email, password, name, role, created_at, updated_at) 
		VALUES ($1, $2, 'pass123', 'GDPR Subject', 'user', '2026-05-18 10:00:00', '2026-05-18 10:00:00')`, 
		userID, emailAddr)
	if err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	// Seed RefreshToken for user
	_, err = db.Exec(`INSERT INTO "bffx_refresh_token" (id, user_id, token, created_at, updated_at) 
		VALUES ('tok_1', $1, 'refresh_token_string_abc', '2026-05-18 10:00:00', '2026-05-18 10:00:00')`,
		userID)
	if err != nil {
		t.Fatalf("seed refresh token failed: %v", err)
	}

	// Seed Device mapping linked to user
	deviceUUID := "device_macbook_pro_99"
	_, err = db.Exec(`INSERT INTO "bffx_device" (id, user_id, device_uuid, created_at, updated_at) 
		VALUES ('dev_1', $1, $2, '2026-05-18 10:00:00', '2026-05-18 10:00:00')`,
		userID, deviceUUID)
	if err != nil {
		t.Fatalf("seed device failed: %v", err)
	}

	// 4. Verify seed state before erasure
	var userCount, refreshCount, deviceCount int
	db.QueryRow(`SELECT COUNT(*) FROM "bffx_user" WHERE id = $1`, userID).Scan(&userCount)
	db.QueryRow(`SELECT COUNT(*) FROM "bffx_refresh_token" WHERE user_id = $1`, userID).Scan(&refreshCount)
	db.QueryRow(`SELECT COUNT(*) FROM "bffx_device" WHERE user_id = $1`, userID).Scan(&deviceCount)
	
	if userCount != 1 || refreshCount != 1 || deviceCount != 1 {
		t.Fatalf("Expected seed counts [1, 1, 1], got [%d, %d, %d]", userCount, refreshCount, deviceCount)
	}

	// 5. Execute Cascading GDPR User Deletion Transaction (Identical to our playbook)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback()

	// A. Delete active refresh tokens
	_, err = tx.ExecContext(context.Background(), `DELETE FROM "bffx_refresh_token" WHERE user_id = $1`, userID)
	if err != nil {
		t.Fatalf("delete refreshtoken failed: %v", err)
	}

	// B. Dissociate devices (preserves hardware identity but breaks the link to personal profile)
	_, err = tx.ExecContext(context.Background(), `UPDATE "bffx_device" SET user_id = NULL WHERE user_id = $1`, userID)
	if err != nil {
		t.Fatalf("dissociate device failed: %v", err)
	}

	// C. Physically delete core user record
	_, err = tx.ExecContext(context.Background(), `DELETE FROM "bffx_user" WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("delete user failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 6. Assert Physical Deletion of User and Sessions
	db.QueryRow(`SELECT COUNT(*) FROM "bffx_user" WHERE id = $1`, userID).Scan(&userCount)
	db.QueryRow(`SELECT COUNT(*) FROM "bffx_refresh_token" WHERE user_id = $1`, userID).Scan(&refreshCount)
	
	if userCount != 0 {
		t.Errorf("expected User to be physically erased, count = %d", userCount)
	}
	if refreshCount != 0 {
		t.Errorf("expected RefreshTokens to be physically erased, count = %d", refreshCount)
	}

	// 7. Assert Device Dissociation (Hardware stats preserved, but user linkage nulled out)
	var linkedUserID sql.NullString
	err = db.QueryRow(`SELECT user_id FROM "bffx_device" WHERE device_uuid = $1`, deviceUUID).Scan(&linkedUserID)
	if err != nil {
		t.Fatalf("querying device post-erasure failed: %v", err)
	}
	if linkedUserID.Valid {
		t.Errorf("expected Device user_id to be NULL, got %q", linkedUserID.String)
	}
}
