package compiler

import "testing"

func TestToCamelCase(t *testing.T) {
	if got := toCamelCase("submit_score"); got != "SubmitScore" {
		t.Fatalf("toCamelCase: got %q", got)
	}
}

func TestToGRPCName(t *testing.T) {
	if got := toGRPCName("Get", "User"); got != "GetUser" {
		t.Fatalf("GetUser: got %q", got)
	}
	if got := toGRPCName("List", "User"); got != "ListUser" {
		t.Fatalf("ListUser: got %q", got)
	}
}
