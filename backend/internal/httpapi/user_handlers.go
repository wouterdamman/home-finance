package httpapi

import (
	"fmt"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"

	"github.com/wouterdamman/home-finance/internal/auth"
)

const maxAvatarUploadBytes = 5 << 20 // 5MB — plenty for a profile photo

func avatarURL(id int64, objectKey *string) *string {
	if objectKey == nil || *objectKey == "" {
		return nil
	}
	url := fmt.Sprintf("/api/users/%d/avatar", id)
	return &url
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var id int64
	var email, displayName, role string
	var avatarKey *string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT id, email, display_name, role, avatar_object_key FROM users WHERE id = $1`, uid).
		Scan(&id, &email, &displayName, &role, &avatarKey); err != nil {
		Error(w, http.StatusUnauthorized, "unauthorized", "user not found")
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"id": id, "email": email, "displayName": displayName, "role": role,
		"avatarUrl": avatarURL(id, avatarKey),
	})
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var body struct {
		DisplayName string `json:"displayName"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE users SET display_name=$2 WHERE id=$1`, uid, body.DisplayName); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	if s.s3 == nil {
		Error(w, http.StatusServiceUnavailable, "s3_not_configured", "object storage is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarUploadBytes)
	if err := r.ParseMultipartForm(maxAvatarUploadBytes); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "file too large or malformed upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "missing file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" {
		Error(w, http.StatusBadRequest, "bad_request", "only png, jpeg or webp images are allowed")
		return
	}

	objectKey := fmt.Sprintf("home-finance/%d", uid)
	ctx := r.Context()
	if _, err := s.s3.PutObject(ctx, s.cfg.S3Bucket, objectKey, file, header.Size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		Error(w, http.StatusInternalServerError, "s3_error", err.Error())
		return
	}
	if _, err := s.pool.Exec(ctx, `UPDATE users SET avatar_object_key=$2 WHERE id=$1`, uid, objectKey); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(ctx, "user.avatar_upload", "user", uid, nil)
	JSON(w, http.StatusOK, map[string]any{"avatarUrl": avatarURL(uid, &objectKey)})
}

// handleGetAvatar streams the avatar through the app rather than exposing
// a public bucket URL — any session-authenticated user can view any
// avatar (display names/photos aren't sensitive), but the object storage
// itself is never reachable without going through this auth-gated proxy.
func (s *Server) handleGetAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if s.s3 == nil {
		Error(w, http.StatusNotFound, "not_found", "no avatar")
		return
	}
	var objectKey *string
	if err := s.pool.QueryRow(r.Context(), `SELECT avatar_object_key FROM users WHERE id=$1`, id).Scan(&objectKey); err != nil || objectKey == nil {
		Error(w, http.StatusNotFound, "not_found", "no avatar")
		return
	}
	obj, err := s.s3.GetObject(r.Context(), s.cfg.S3Bucket, *objectKey, minio.GetObjectOptions{})
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "no avatar")
		return
	}
	defer obj.Close()
	stat, err := obj.Stat()
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "no avatar")
		return
	}
	w.Header().Set("Content-Type", stat.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	io.Copy(w, obj)
}

// ── Admin: user management ──────────────────────────────────────────

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID          int64   `json:"id"`
		Email       string  `json:"email"`
		DisplayName string  `json:"displayName"`
		Role        string  `json:"role"`
		AvatarURL   *string `json:"avatarUrl,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id, email, display_name, role, avatar_object_key FROM users ORDER BY email`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var avatarKey *string
		if err := rows.Scan(&ro.ID, &ro.Email, &ro.DisplayName, &ro.Role, &avatarKey); err != nil {
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		ro.AvatarURL = avatarURL(ro.ID, avatarKey)
		out = append(out, ro)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.Role != "admin" && body.Role != "user" {
		Error(w, http.StatusBadRequest, "bad_request", "role must be admin or user")
		return
	}

	ctx := r.Context()
	if body.Role == "user" {
		var adminCount int
		if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&adminCount); err != nil {
			Error(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		var currentRole string
		if err := s.pool.QueryRow(ctx, `SELECT role FROM users WHERE id=$1`, id).Scan(&currentRole); err != nil {
			Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		if currentRole == "admin" && adminCount <= 1 {
			Error(w, http.StatusConflict, "last_admin", "cannot demote the last remaining admin")
			return
		}
	}

	if _, err := s.pool.Exec(ctx, `UPDATE users SET role=$2 WHERE id=$1`, id, body.Role); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(ctx, "user.role_change", "user", id, map[string]any{"role": body.Role})
	w.WriteHeader(http.StatusNoContent)
}
