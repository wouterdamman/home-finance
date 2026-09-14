package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registers the jpeg decoder used by handleUploadAvatar's content sniff
	_ "image/png"  // registers the png decoder used by handleUploadAvatar's content sniff
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/wouterdamman/home-finance/internal/auth"
)

const maxAvatarUploadBytes = 10 << 20 // 10MB — plenty for a profile photo

// maxDisplayNameLen bounds a self-chosen display name.
const maxDisplayNameLen = 200

func avatarURL(id int64, hasAvatar *string) *string {
	if hasAvatar == nil || *hasAvatar == "" {
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
	var avatarContentType *string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT id, email, display_name, role, avatar_content_type FROM users WHERE id = $1`, uid).
		Scan(&id, &email, &displayName, &role, &avatarContentType); err != nil {
		Error(w, http.StatusUnauthorized, "unauthorized", "user not found")
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"id": id, "email": email, "displayName": displayName, "role": role,
		"avatarUrl": avatarURL(id, avatarContentType),
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
		badRequest(w, err)
		return
	}
	// display_name is an unbounded TEXT column rendered in every user list
	// and audit-log row, so it has to be bounded here.
	displayName := strings.TrimSpace(body.DisplayName)
	if displayName == "" {
		Error(w, http.StatusBadRequest, "bad_request", "displayName is required")
		return
	}
	if len([]rune(displayName)) > maxDisplayNameLen {
		Error(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("displayName must be at most %d characters", maxDisplayNameLen))
		return
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE users SET display_name=$2 WHERE id=$1`, uid, displayName)
	if err != nil {
		dbError(w, "updateMe", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	s.auditLog(r.Context(), "user.profile_update", "user", uid, map[string]any{"displayName": displayName})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
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
	if contentType != "image/png" && contentType != "image/jpeg" {
		Error(w, http.StatusBadRequest, "bad_request", "only png or jpeg images are allowed")
		return
	}

	ctx := r.Context()
	data, err := io.ReadAll(file)
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "failed to read upload")
		return
	}

	// The declared Content-Type above is client-supplied and easily spoofed
	// (e.g. a script or executable uploaded with a fake "image/png" header).
	// Decode the actual bytes and derive the stored content type from what
	// the file really is, so nothing but a genuinely valid jpeg/png is ever
	// persisted or served back to a browser.
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "file is not a valid image")
		return
	}
	switch format {
	case "jpeg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	default:
		Error(w, http.StatusBadRequest, "bad_request", "only png or jpeg images are allowed")
		return
	}

	if err := s.avatarStorage.Put(ctx, uid, contentType, data); err != nil {
		dbError(w, "avatarStorage.Put", err)
		return
	}
	s.auditLog(ctx, "user.avatar_upload", "user", uid, nil)
	JSON(w, http.StatusOK, map[string]any{"avatarUrl": avatarURL(uid, &contentType)})
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
	contentType, data, err := s.avatarStorage.Get(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "no avatar")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Write(data)
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
	rows, err := s.pool.Query(r.Context(), `SELECT id, email, display_name, role, avatar_content_type FROM users ORDER BY email`)
	if err != nil {
		dbError(w, "listUsers", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var avatarContentType *string
		if err := rows.Scan(&ro.ID, &ro.Email, &ro.DisplayName, &ro.Role, &avatarContentType); err != nil {
			dbError(w, "listUsers.scan", err)
			return
		}
		ro.AvatarURL = avatarURL(ro.ID, avatarContentType)
		out = append(out, ro)
	}
	// Without this a mid-iteration failure returns 200 with a silently
	// truncated list — on the one screen where "who is an admin" matters.
	if err := rows.Err(); err != nil {
		dbError(w, "listUsers", err)
		return
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
		badRequest(w, err)
		return
	}
	if body.Role != "admin" && body.Role != "user" {
		Error(w, http.StatusBadRequest, "bad_request", "role must be admin or user")
		return
	}

	ctx := r.Context()
	// "At least one admin must remain" cannot be expressed as a constraint, and
	// a read-then-write cannot enforce it either: as three separate round trips
	// two concurrent demotes (one admin double-clicking is enough) both read
	// adminCount = 2, both passed, and the app was left with no admin and no
	// way back in short of direct database access. Folding the count into the
	// UPDATE's WHERE clause is not sufficient either — under READ COMMITTED the
	// subquery still reads a pre-statement snapshot.
	//
	// So: lock the admin set for the duration of the transaction. Ordering by
	// id gives every caller the same lock order, and the target row is locked
	// after the admin set, so concurrent demotes serialize instead of racing.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		dbError(w, "updateUserRole.begin", err)
		return
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE role='admin' ORDER BY id FOR UPDATE`)
	if err != nil {
		dbError(w, "updateUserRole.lockAdmins", err)
		return
	}
	var adminCount int
	for rows.Next() {
		adminCount++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		dbError(w, "updateUserRole.lockAdmins", err)
		return
	}

	var currentRole string
	err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&currentRole)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		dbError(w, "updateUserRole.lockTarget", err)
		return
	}

	if currentRole == "admin" && body.Role != "admin" && adminCount <= 1 {
		Error(w, http.StatusConflict, "last_admin", "cannot demote the last remaining admin")
		return
	}

	if currentRole != body.Role {
		if _, err := tx.Exec(ctx, `UPDATE users SET role=$2 WHERE id=$1`, id, body.Role); err != nil {
			dbError(w, "updateUserRole", err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, "updateUserRole.commit", err)
		return
	}
	s.auditLog(ctx, "user.role_change", "user", id, map[string]any{"from": currentRole, "to": body.Role})
	w.WriteHeader(http.StatusNoContent)
}
