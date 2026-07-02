package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"smartdb/internal/auth"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
)

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type CreateAPIKeyResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Token string `json:"token"`
}

func CreateAPIKeyHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req CreateAPIKeyRequest) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}
		if req.Name == "" {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_KEY_NAME", "Key name is required")
			return
		}

		role := auth.Role(req.Role)
		switch role {
		case auth.RoleAdmin, auth.RoleReadWrite, auth.RoleReadOnly:
		default:
			handler.WriteError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be admin, read_write, or read_only")
			return
		}

		ac := auth.GetAuthContext(r.Context())
		if ac != nil && ac.Role != auth.RoleAdmin && ac.Role != auth.RoleSystem {
			handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Only admin or system keys can create new API keys")
			return
		}

		plaintext, err := auth.GenerateToken()
		if err != nil {
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate token")
			return
		}
		hash := auth.HashToken(plaintext)

		id, err := auth.CreateKey(App.SystemDB, &projectID, req.Name, hash, role)
		if err != nil {
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key")
			return
		}

		handler.WriteJSON(w, http.StatusCreated, CreateAPIKeyResponse{
			ID:    id,
			Name:  req.Name,
			Role:  req.Role,
			Token: plaintext,
		})
	})
}

type APIKeyListItem struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	CreatedAt string  `json:"created_at"`
	RevokedAt *string `json:"revoked_at,omitempty"`
}

func ListAPIKeysHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		keys, err := auth.ListKeys(App.SystemDB, &projectID)
		if err != nil {
			handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list API keys")
			return
		}

		items := make([]APIKeyListItem, len(keys))
		for i, k := range keys {
			items[i] = APIKeyListItem{
				ID:        k.ID,
				Name:      k.Name,
				Role:      string(k.Role),
				CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z"),
			}
			if k.RevokedAt != nil {
				s := k.RevokedAt.Format("2006-01-02T15:04:05Z")
				items[i].RevokedAt = &s
			}
		}

		handler.WriteJSON(w, http.StatusOK, items)
	}
}

func RevokeAPIKeyHandler(App *domain.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("project")
		if err := handler.ValidateProjectID(projectID); err != nil {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", err.Error())
			return
		}

		keyID := r.PathValue("id")
		if keyID == "" {
			handler.WriteError(w, http.StatusBadRequest, "INVALID_KEY_ID", "Key ID is required")
			return
		}

		err := auth.RevokeKeyForProject(App.SystemDB, keyID, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				handler.WriteError(w, http.StatusNotFound, "KEY_NOT_FOUND", "API key not found or already revoked")
			} else {
				handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke API key")
			}
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
