package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestMergeGuestIntoAccount_DeviceOnly(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	reg := &manifest.Registry{}

	guest, _ := store.Create(ctx, "User", map[string]any{"role": "guest", "device_id": "dev-merge"})
	acct, _ := store.Create(ctx, "User", map[string]any{"role": "user", "email": "a@b.co"})
	guestID := fmt.Sprintf("%v", guest["id"])
	acctID := fmt.Sprintf("%v", acct["id"])

	MergeGuestIntoAccount(ctx, store, reg, guestID, acctID)

	if _, err := store.Get(ctx, "User", guestID); err == nil {
		t.Fatal("guest user should be deleted")
	}
	u, err := store.Get(ctx, "User", acctID)
	if err != nil {
		t.Fatal("account missing")
	}
	if fmt.Sprintf("%v", u["device_id"]) != "dev-merge" {
		t.Fatalf("device_id not merged: %v", u["device_id"])
	}
}

func TestMergeGuestIntoAccount_ListByOwner(t *testing.T) {
	ctx := context.Background()
	raw := `apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: Note
spec:
  fields:
    - { name: body, type: string }
  telemetry: false
`
	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	reg := &manifest.Registry{Resources: []*manifest.Manifest{&m}}

	guest, _ := store.Create(ctx, "User", map[string]any{"role": "guest", "device_id": "dev-x"})
	acct, _ := store.Create(ctx, "User", map[string]any{"role": "user", "email": "a@b.co"})
	guestID := fmt.Sprintf("%v", guest["id"])
	acctID := fmt.Sprintf("%v", acct["id"])

	store.Create(ctx, "Note", map[string]any{"body": "hi", "created_by": guestID})

	n := MergeGuestIntoAccount(ctx, store, reg, guestID, acctID)
	if n != 1 {
		t.Fatalf("expected 1 row reassigned, got %d", n)
	}
	notes, _ := store.List(ctx, "Note", 10, 0)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	note := notes[0]
	if fmt.Sprintf("%v", note["created_by"]) != acctID {
		t.Fatalf("created_by = %v, want %v", note["created_by"], acctID)
	}
}

func TestMergeGuestIntoAccount_SecondCallNoop(t *testing.T) {
	ctx := context.Background()
	raw := `apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: Note
spec:
  fields:
    - { name: body, type: string }
  telemetry: false
`
	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	reg := &manifest.Registry{Resources: []*manifest.Manifest{&m}}

	guest, _ := store.Create(ctx, "User", map[string]any{"role": "guest", "device_id": "dev-y"})
	acct, _ := store.Create(ctx, "User", map[string]any{"role": "user", "email": "b@b.co"})
	guestID := fmt.Sprintf("%v", guest["id"])
	acctID := fmt.Sprintf("%v", acct["id"])
	store.Create(ctx, "Note", map[string]any{"body": "x", "created_by": guestID})

	n1 := MergeGuestIntoAccount(ctx, store, reg, guestID, acctID)
	if n1 != 1 {
		t.Fatalf("first merge: want 1 row, got %d", n1)
	}
	n2 := MergeGuestIntoAccount(ctx, store, reg, guestID, acctID)
	if n2 != 0 {
		t.Fatalf("second merge after guest deleted: want 0 rows, got %d", n2)
	}
}
