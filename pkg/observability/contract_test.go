package observability

import "testing"

func TestIsCanonicalLogProvider(t *testing.T) {
	for _, name := range []string{LogProviderSlog, LogProviderAxiom, LogProviderSentry} {
		if !IsCanonicalLogProvider(name) {
			t.Fatalf("expected %q to be canonical", name)
		}
	}
	if IsCanonicalLogProvider("datadog") {
		t.Fatal("datadog is not a log provider battery value")
	}
}

func TestEntryFromAdminAudit(t *testing.T) {
	entry := EntryFromAdminAudit("user-1", "admin", "delete", "Resource", "res-9", `{"reason":"test"}`, "127.0.0.1")
	if entry.UserID != "user-1" || entry.Action != "delete" || entry.Resource != "Resource" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Metadata["actor_type"] != "admin" {
		t.Fatalf("expected actor_type metadata")
	}
}
