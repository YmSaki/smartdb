package v1

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"smartdb/internal/auth"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
	"smartdb/internal/project"
)

// WipeProjectHandler permanently purges a project: it revokes every API key
// issued for it and removes its on-disk directory (database, backups,
// migrations). It only operates on projects already in the "deleted" state,
// and is restricted to system keys (see RequireSystemKey) since it is a
// fleet-management operation, not something any project-scoped key —
// even an "admin" one — should be able to trigger on itself.
func WipeProjectHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		current, err := project.GetProject(App.SystemDB, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
			} else {
				handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
			return
		}

		if !current.State.CanTransitionTo(domain.StateWiped) {
			handler.WriteError(w, http.StatusConflict, "INVALID_TRANSITION",
				fmt.Sprintf("Cannot wipe a project in state %s (must be %s first)", current.State, domain.StateDeleted))
			return
		}

		if _, err := auth.RevokeAllKeysForProject(App.SystemDB, projectID); err != nil {
			slog.Error("wipe: failed to revoke project keys", "projectID", projectID, "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke project keys")
			return
		}

		projectPath := filepath.Join(App.Config.DataDir, projectID)
		if err := os.RemoveAll(projectPath); err != nil {
			slog.Error("wipe: failed to remove project directory", "projectID", projectID, "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to remove project data")
			return
		}

		if err := project.UpdateProjectState(App.SystemDB, projectID, domain.StateWiped); err != nil {
			slog.Error("wipe: failed to update project state", "projectID", projectID, "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
