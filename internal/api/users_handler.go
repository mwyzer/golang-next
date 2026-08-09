package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"golang-nextjs/internal/db"
)

type UsersHandler struct {
	Users *db.UserRepo
}

var validUserRoles = []string{"uploader", "reviewer", "admin"}

type createUserRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Create implements POST /users (admin-only): provisions a new user
// under the caller's tenant and returns a freshly generated bearer
// token. The token is shown exactly once, in this response — only its
// hash is persisted, so it can't be recovered afterward and must be
// rotated (not retrieved) if lost.
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "could not parse request body: "+err.Error())
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "MISSING_EMAIL", "email is required")
		return
	}
	if !slices.Contains(validUserRoles, req.Role) {
		writeError(w, http.StatusBadRequest, "INVALID_ROLE", `role must be one of "uploader", "reviewer", "admin"`)
		return
	}

	user, token, err := h.Users.Create(r.Context(), tenantID, req.Email, req.Role)
	if errors.Is(err, db.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "a user with this email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"token":   token,
	})
}
