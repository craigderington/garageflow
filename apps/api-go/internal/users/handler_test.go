package users

import (
	"testing"

	"github.com/garageflow/api-go/internal/types"
)

func TestCanAssignRole(t *testing.T) {
	if canAssignRole("admin", types.RoleOwner) {
		t.Fatal("admin must not be able to create an owner")
	}
	if !canAssignRole("owner", types.RoleOwner) {
		t.Fatal("owner should be able to create an owner")
	}
	if canAssignRole("owner", types.Role("superuser")) {
		t.Fatal("unknown role accepted")
	}
}
