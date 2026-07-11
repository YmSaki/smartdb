package auth

import (
	"fmt"
	"smartdb/internal/project"
)

func CheckSQLPermission(role Role, sqlType project.SQLType) error {
	switch role {
	case RoleSystem:
		return fmt.Errorf("system key cannot execute project SQL operations")
	case RoleAdmin:
		return nil
	case RoleReadWrite:
		switch sqlType {
		case project.SQLTypeRead, project.SQLTypeEdit:
			return nil
		default:
			return fmt.Errorf("read_write key cannot execute %s operations", sqlType)
		}
	case RoleReadOnly:
		switch sqlType {
		case project.SQLTypeRead:
			return nil
		default:
			return fmt.Errorf("read_only key cannot execute %s operations", sqlType)
		}
	default:
		return fmt.Errorf("unknown role: %s", role)
	}
}
