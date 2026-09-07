package router

import "testing"

func TestNormalizeCRUDPagination(t *testing.T) {
	t.Parallel()
	l, o := normalizeCRUDPagination("2000", "-5")
	if l != 100 || o != 0 {
		t.Fatalf("clamp large limit / negative offset: got limit=%d offset=%d", l, o)
	}
	l, o = normalizeCRUDPagination("", "")
	if l != 20 || o != 0 {
		t.Fatalf("defaults: got limit=%d offset=%d", l, o)
	}
	l, o = normalizeCRUDPagination("10", "5")
	if l != 10 || o != 5 {
		t.Fatalf("explicit: got limit=%d offset=%d", l, o)
	}
	l, o = normalizeCRUDPagination("50", "2000001")
	if l != 50 || o != 1_000_000 {
		t.Fatalf("offset cap: got limit=%d offset=%d", l, o)
	}
}
