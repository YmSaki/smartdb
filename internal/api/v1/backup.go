package v1

import (
	"net/http"
	"smartdb/internal/auth"
	"smartdb/internal/backup"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
)

func BackupHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		ac := auth.GetAuthContext(r.Context())
		if ac != nil && ac.Role != auth.RoleAdmin {
			handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Only admin keys can create backups")
			return
		}

		release, ok := App.ProjectLocks.TryReadLock(projectID)
		if !ok {
			handler.WriteError(w, http.StatusConflict, "PROJECT_LOCKED", "A restore or migration is in progress for this project")
			return
		}
		defer release()

		path, err := backup.CreateBackup(App.Config.DataDir, projectID)
		if err != nil {
			handler.WriteError(w, http.StatusInternalServerError, "BACKUP_FAILED", "Backup failed: "+err.Error())
			return
		}

		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"backup": path,
		})
	}
}

type RestoreRequest struct {
	Backup string `json:"backup"`
}

func RestoreHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req RestoreRequest) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}
		if req.Backup == "" {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_BACKUP", "Backup name is required")
			return
		}

		ac := auth.GetAuthContext(r.Context())
		if ac != nil && ac.Role != auth.RoleAdmin {
			handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Only admin keys can restore from a backup")
			return
		}

		release, ok := App.ProjectLocks.TryWriteLock(projectID)
		if !ok {
			handler.WriteError(w, http.StatusConflict, "PROJECT_LOCKED", "Another backup, restore, or migration is in progress for this project")
			return
		}
		defer release()

		if err := backup.RestoreBackup(App.Config.DataDir, projectID, req.Backup); err != nil {
			handler.WriteError(w, http.StatusInternalServerError, "RESTORE_FAILED", "Restore failed: "+err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
