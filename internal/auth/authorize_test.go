package auth

import (
	"smartdb/internal/project"
	"testing"
)

func TestCheckSQLPermission(t *testing.T) {
	tests := []struct {
		name    string
		role    Role
		sqlType project.SQLType
		wantErr bool
	}{
		{"admin can read", RoleAdmin, project.SQLTypeRead, false},
		{"admin can edit", RoleAdmin, project.SQLTypeEdit, false},
		{"admin can manage", RoleAdmin, project.SQLTypeManage, false},
		{"admin can admin", RoleAdmin, project.SQLTypeAdmin, false},

		{"read_write can read", RoleReadWrite, project.SQLTypeRead, false},
		{"read_write can edit", RoleReadWrite, project.SQLTypeEdit, false},
		{"read_write can manage", RoleReadWrite, project.SQLTypeManage, false},
		{"read_write cannot admin", RoleReadWrite, project.SQLTypeAdmin, true},

		{"read_only can read", RoleReadOnly, project.SQLTypeRead, false},
		{"read_only cannot edit", RoleReadOnly, project.SQLTypeEdit, true},
		{"read_only cannot manage", RoleReadOnly, project.SQLTypeManage, true},
		{"read_only cannot admin", RoleReadOnly, project.SQLTypeAdmin, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckSQLPermission(tt.role, tt.sqlType)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckSQLPermission(%s, %s) error = %v, wantErr %v", tt.role, tt.sqlType, err, tt.wantErr)
			}
		})
	}
}

func TestCheckSQLPermissionMatrix(t *testing.T) {
	roles := []Role{RoleAdmin, RoleReadWrite, RoleReadOnly}
	sqlTypes := []project.SQLType{project.SQLTypeRead, project.SQLTypeEdit, project.SQLTypeManage, project.SQLTypeAdmin}

	allowed := map[Role]map[project.SQLType]bool{
		RoleAdmin:     {project.SQLTypeRead: true, project.SQLTypeEdit: true, project.SQLTypeManage: true, project.SQLTypeAdmin: true},
		RoleReadWrite: {project.SQLTypeRead: true, project.SQLTypeEdit: true, project.SQLTypeManage: true, project.SQLTypeAdmin: false},
		RoleReadOnly:  {project.SQLTypeRead: true, project.SQLTypeEdit: false, project.SQLTypeManage: false, project.SQLTypeAdmin: false},
	}

	for _, role := range roles {
		for _, st := range sqlTypes {
			t.Run(string(role)+" "+string(st), func(t *testing.T) {
				err := CheckSQLPermission(role, st)
				shouldAllow := allowed[role][st]
				if shouldAllow && err != nil {
					t.Errorf("should allow %s+%s but got error: %v", role, st, err)
				}
				if !shouldAllow && err == nil {
					t.Errorf("should deny %s+%s but got nil error", role, st)
				}
			})
		}
	}
}

func TestCheckSQLPermissionSystemRoleDenied(t *testing.T) {
	for _, st := range []project.SQLType{
		project.SQLTypeRead, project.SQLTypeEdit, project.SQLTypeManage, project.SQLTypeAdmin,
	} {
		if err := CheckSQLPermission(RoleSystem, st); err == nil {
			t.Errorf("system role should never be allowed to run project SQL (%s)", st)
		}
	}
}

func TestCheckSQLPermissionInvalidRole(t *testing.T) {
	err := CheckSQLPermission(Role("superuser"), project.SQLTypeRead)
	if err == nil {
		t.Error("unknown role should be denied")
	}

	err = CheckSQLPermission(Role(""), project.SQLTypeRead)
	if err == nil {
		t.Error("empty role should be denied")
	}
}
