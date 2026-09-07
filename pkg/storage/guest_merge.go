package storage

import (
	"context"
	"fmt"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

const mergeListCap = 50000

// MergeGuestIntoAccount moves rows owned by guestUserID onto accountUserID, copies device_id onto the
// account row when missing, then deletes the guest User. Skips telemetry resources and the User resource
// table until the final delete. Safe no-op when IDs are equal or empty.
func MergeGuestIntoAccount(ctx context.Context, store Store, reg *manifest.Registry, guestUserID, accountUserID string) int {
	if guestUserID == "" || accountUserID == "" || guestUserID == accountUserID || reg == nil {
		return 0
	}

	guest, errG := store.Get(ctx, "User", guestUserID)
	acc, errA := store.Get(ctx, "User", accountUserID)
	if errG == nil && errA == nil {
		gDev, _ := guest["device_id"].(string)
		aDev, _ := acc["device_id"].(string)
		if gDev != "" && aDev == "" {
			store.Update(ctx, "User", accountUserID, map[string]any{"device_id": gDev})
		}
	}

	n := 0
	for _, m := range reg.Resources {
		name := m.Metadata.Name
		if name == "User" {
			continue
		}
		var spec manifest.ResourceSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}
		if spec.Telemetry {
			continue
		}

		rowsByOwner, err := store.ListByOwner(ctx, name, guestUserID, mergeListCap, 0)
		if err == nil {
			for _, row := range rowsByOwner {
				id := fmt.Sprintf("%v", row["id"])
				if _, err := store.Update(ctx, name, id, map[string]any{"created_by": accountUserID}); err == nil {
					n++
				}
			}
		}

		hasUserID := false
		for _, f := range spec.Fields {
			if f.Name == "user_id" {
				hasUserID = true
				break
			}
		}
		if !hasUserID {
			continue
		}
		rowsByQuery, err := store.Query(ctx, name).Where("user_id", "=", guestUserID).Limit(mergeListCap).Execute(ctx)
		if err == nil {
			for _, row := range rowsByQuery {
				id := fmt.Sprintf("%v", row["id"])
				if _, err := store.Update(ctx, name, id, map[string]any{"user_id": accountUserID}); err == nil {
					n++
				}
			}
		}
	}

	store.Delete(ctx, "User", guestUserID)
	return n
}
