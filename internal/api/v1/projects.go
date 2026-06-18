package v1

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"smartdb/internal/domain"
	"smartdb/internal/project"
)

type CreateProjectRequest struct {
	Name string `json:"name"`
}

type CreateProjectResponse struct {
	ProjectID string `json:"projectID"`
}

func CreateProjectHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		var req CreateProjectRequest

		err := json.NewDecoder(r.Body).Decode(&req)

		if err != nil {
			http.Error(
				w,
				"Invalid request body",
				http.StatusBadRequest,
			)
			return
		}

		if req.Name == "" {
			http.Error(
				w,
				"name is required",
				http.StatusBadRequest,
			)
			return
		}

		projectID, err := project.Create(req.Name, App.SystemDB)

		if err != nil {
			slog.Error(err.Error())
			http.Error(
				w,
				"Project creation failed.",
				http.StatusInternalServerError,
			)
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(
			CreateProjectResponse{
				ProjectID: projectID,
			},
		)
	}
}

func GetProjectsHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		filter := project.ProjectFilter{}
		filter.State = []domain.ProjectState{domain.StateInactive, domain.StateActive}
		list, err := project.GetProjectList(App.SystemDB, filter)
		if err != nil {
			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
		jsonData, err := json.Marshal(list)
		if err != nil {
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(jsonData)
	}
}

func GetProjectDetailHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		projectID := r.PathValue("project")

		projectData, err := project.GetProject(App.SystemDB, projectID)

		w.Header().Set(
			"Content-Type",
			"application/json",
		)

		jsonData, err := json.Marshal(projectData)
		if err != nil {
			slog.Warn("json convert error", "error", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jsonData)
	}
}

// RemoveProjectHandler 実はstateをdeletedに変えるだけという。wipeの時まで実は実態が残る。
func RemoveProjectHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		projectID := r.PathValue("project")
		err := project.UpdateProjectState(App.SystemDB, projectID, domain.StateDeleted)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				slog.Warn("A non-existent project name was specified.")
				w.WriteHeader(http.StatusNotFound)

			default:
				slog.Error("Project Remove Error", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func QueryExecuteHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		r.PathValue("project")

		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.WriteHeader(http.StatusOK)
	}
}
