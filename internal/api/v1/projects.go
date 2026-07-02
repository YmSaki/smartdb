package v1

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"smartdb/internal/auth"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
	"smartdb/internal/project"
)

type CreateProjectRequest struct {
	Name string `json:"name"`
}

type CreateProjectResponse struct {
	ProjectID string `json:"projectID"`
}

func CreateProjectHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req CreateProjectRequest) {
		if err := handler.ValidateProjectName(req.Name); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_NAME", err.Error())
			return
		}

		projectID, err := project.Create(req.Name, App.SystemDB, App.Config.DataDir)
		if err != nil {
			slog.Error("project creation failed", "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "PROJECT_CREATION_FAILED", "Project creation failed")
			return
		}

		handler.WriteJSON(w, http.StatusCreated, CreateProjectResponse{ProjectID: projectID})
	})
}

func GetProjectsHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := project.ProjectFilter{}
		filter.State = []domain.ProjectState{domain.StateInactive, domain.StateActive}
		list, err := project.GetProjectList(App.SystemDB, filter)
		if err != nil {
			slog.Error("failed to list projects", "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list projects")
			return
		}
		handler.WriteJSON(w, http.StatusOK, list)
	}
}

func GetProjectDetailHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		projectData, err := project.GetProject(App.SystemDB, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
			} else {
				handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
			return
		}

		handler.WriteJSON(w, http.StatusOK, projectData)
	}
}

// RemoveProjectHandler sets state to deleted (actual wipe happens via
// WipeProjectHandler, see wipe.go).
func RemoveProjectHandler(App *domain.App) http.HandlerFunc {
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

		if !current.State.CanTransitionTo(domain.StateDeleted) {
			handler.WriteError(w, http.StatusConflict, "INVALID_TRANSITION",
				fmt.Sprintf("Cannot transition from %s to %s", current.State, domain.StateDeleted))
			return
		}

		err = project.UpdateProjectState(App.SystemDB, projectID, domain.StateDeleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				slog.Warn("A non-existent project name was specified.")
				handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
			} else {
				slog.Error("Project Remove Error", "error", err)
				handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type PatchProjectRequest struct {
	Name  string `json:"name,omitempty"`
	State string `json:"state,omitempty"`
}

func PatchProjectHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req PatchProjectRequest) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		if req.Name != "" {
			if err := handler.ValidateProjectName(req.Name); err != nil {
				handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_NAME", err.Error())
				return
			}
			if err := project.UpdateProjectName(App.SystemDB, projectID, req.Name); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
				} else {
					handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
				}
				return
			}
		}

		if req.State != "" {
			newState := domain.ProjectState(req.State)
			if !newState.IsValid() {
				handler.WriteError(w, http.StatusBadRequest, "INVALID_STATE", "Invalid project state")
				return
			}

			// Deletion/wipe are fleet-management operations restricted to
			// system keys via their own endpoints (DELETE .../{project} and
			// POST .../{project}/wipe). PATCH — reachable by project-scoped
			// keys too — must not be usable to reach the same states.
			switch newState {
			case domain.StateDeleted, domain.StateDeleting, domain.StateWiped:
				handler.WriteError(w, http.StatusForbidden, "FORBIDDEN",
					"Use DELETE /projects/{project} or /wipe to change to this state")
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

			if !current.State.CanTransitionTo(newState) {
				handler.WriteError(w, http.StatusConflict, "INVALID_TRANSITION",
					fmt.Sprintf("Cannot transition from %s to %s", current.State, newState))
				return
			}

			if err := project.UpdateProjectState(App.SystemDB, projectID, newState); err != nil {
				handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
				return
			}
		}

		projectData, err := project.GetProject(App.SystemDB, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
			} else {
				handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
			return
		}

		handler.WriteJSON(w, http.StatusOK, projectData)
	})
}

type ProjectStatsResponse struct {
	Size             int64  `json:"size"`
	Tables           int    `json:"tables"`
	BackupCount      int    `json:"backup_count"`
	MigrationVersion string `json:"migration_version"`
}

func GetProjectStatsHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), App.Config.QueryTimeout)
		defer cancel()

		stats, err := project.GetProjectStats(ctx, App.Config.DataDir, projectID)
		if err != nil {
			slog.Error("failed to get project stats", "projectID", projectID, "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get project stats")
			return
		}

		handler.WriteJSON(w, http.StatusOK, stats)
	}
}

type TablesResponse struct {
	Tables []string `json:"tables"`
}

func GetProjectTablesHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), App.Config.QueryTimeout)
		defer cancel()

		tables, err := project.GetTables(ctx, App.Config.DataDir, projectID)
		if err != nil {
			slog.Error("failed to get tables", "projectID", projectID, "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get tables")
			return
		}

		handler.WriteJSON(w, http.StatusOK, TablesResponse{Tables: tables})
	}
}

type TableSchemaResponse struct {
	Schema []project.ColumnInfo `json:"schema"`
}

func GetTableSchemaHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		tableName := r.PathValue("table")
		if tableName == "" {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_TABLE_NAME", "Table name must not be empty")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), App.Config.QueryTimeout)
		defer cancel()

		schema, err := project.GetTableSchema(ctx, App.Config.DataDir, projectID, tableName)
		if err != nil {
			slog.Error("failed to get table schema", "projectID", projectID, "table", tableName, "error", err)
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get table schema")
			return
		}

		handler.WriteJSON(w, http.StatusOK, TableSchemaResponse{Schema: schema})
	}
}

type ExecuteSQLRequest struct {
	SQL string `json:"sql"`
}

type ExecuteSQLResponse struct {
	IsSuccess bool `json:"success"`
	Result    struct {
		Rows         []map[string]any `json:"rows"`
		AffectedRows int64            `json:"affectedRows"`
	} `json:"result"`
}

func ExecuteSQLHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req ExecuteSQLRequest) {
		projectId := r.PathValue("project")
		if err := handler.ValidateProjectID(projectId); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		queryType, err := project.QueryJudge(req.SQL)
		if err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_SQL", err.Error())
			return
		}

		ac := auth.GetAuthContext(r.Context())
		if ac != nil {
			if err := auth.CheckSQLPermission(ac.Role, queryType); err != nil {
				handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
				return
			}
		}

		release, ok := App.ProjectLocks.TryReadLock(projectId)
		if !ok {
			handler.WriteError(w, http.StatusConflict, "PROJECT_LOCKED", "A restore or migration is in progress for this project")
			return
		}
		defer release()

		ctx, cancel := context.WithTimeout(r.Context(), App.Config.QueryTimeout)
		defer cancel()

		var responseData = ExecuteSQLResponse{IsSuccess: true}

		switch queryType {
		case project.SQLTypeRead, project.SQLTypeManage:
			qMap, err := project.Query(ctx, App.Config.DataDir, projectId, req.SQL)
			if err != nil {
				handler.WriteError(w, http.StatusBadRequest, "SQL_ERROR", err.Error())
				responseData.IsSuccess = false
				return
			}
			responseData.Result.Rows = qMap
		default:
			aRows, err := project.Execute(ctx, App.Config.DataDir, projectId, req.SQL)
			if err != nil {
				handler.WriteError(w, http.StatusBadRequest, "SQL_ERROR", err.Error())
				responseData.IsSuccess = false
				return
			}
			responseData.Result.AffectedRows = aRows
		}

		handler.WriteJSON(w, http.StatusOK, responseData)
	})
}
