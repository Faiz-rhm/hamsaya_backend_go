package models

import "testing"

func TestUserRole_HasRole(t *testing.T) {
	cases := []struct {
		holder, required UserRole
		want             bool
	}{
		{RoleSuperAdmin, RoleSuperAdmin, true},
		{RoleSuperAdmin, RoleAdmin, true},
		{RoleSuperAdmin, RoleModerator, true},
		{RoleSuperAdmin, RoleUser, true},

		{RoleAdmin, RoleSuperAdmin, false},
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleModerator, true},
		{RoleAdmin, RoleUser, true},

		{RoleModerator, RoleAdmin, false},
		{RoleModerator, RoleModerator, true},
		{RoleModerator, RoleUser, true},

		{RoleUser, RoleModerator, false},
		{RoleUser, RoleUser, true},
	}
	for _, c := range cases {
		got := c.holder.HasRole(c.required)
		if got != c.want {
			t.Errorf("(%s).HasRole(%s) = %v, want %v", c.holder, c.required, got, c.want)
		}
	}
}

func TestUser_IsSuperAdmin(t *testing.T) {
	u := &User{Role: RoleSuperAdmin}
	if !u.IsSuperAdmin() {
		t.Fatal("super_admin user should report IsSuperAdmin")
	}
	if !u.IsAdmin() {
		t.Fatal("super_admin user should also satisfy IsAdmin")
	}
	if !u.IsAdminOrModerator() {
		t.Fatal("super_admin user should satisfy IsAdminOrModerator")
	}
}

func TestUser_AdminInheritance(t *testing.T) {
	regular := &User{Role: RoleAdmin}
	if regular.IsSuperAdmin() {
		t.Fatal("plain admin must not be super_admin")
	}
	if !regular.IsAdmin() {
		t.Fatal("plain admin must satisfy IsAdmin")
	}
}
