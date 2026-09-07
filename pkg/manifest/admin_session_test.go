package manifest

import (
	"testing"
	"time"
)

func TestResolveAdminSession_DevUnlimited(t *testing.T) {
	t.Setenv("BFFX_ENV", "development")
	site := AdminSiteSession{DevUnlimited: boolPtr(true)}
	r := ResolveAdminSession(site)
	if !r.DevUnlimited || r.AbsoluteTTL != 0 {
		t.Fatalf("dev unlimited: %+v", r)
	}
}

func TestResolveAdminSession_Production(t *testing.T) {
	t.Setenv("BFFX_ENV", "production")
	site := AdminSiteSession{DevUnlimited: boolPtr(true), MaxAge: "8h", IdleTimeout: "45m"}
	r := ResolveAdminSession(site)
	if r.DevUnlimited {
		t.Fatal("production must not be dev unlimited")
	}
	if r.AbsoluteTTL != 8*time.Hour || r.IdleTimeout != 45*time.Minute {
		t.Fatalf("prod policy: %+v", r)
	}
}

func boolPtr(b bool) *bool { return &b }
