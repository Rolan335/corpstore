package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corpstore/internal/fileaccess"
	"corpstore/internal/files"
	"corpstore/internal/usecase"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	filesUC *usecase.Files
	authUC  *usecase.Auth
}

func NewHandler(filesUC *usecase.Files, authUC *usecase.Auth) *Handler {
	return &Handler{filesUC: filesUC, authUC: authUC}
}

func (h *Handler) respondError(c *gin.Context, status int, publicMsg string, err error) {
	if err != nil {
		log.Printf("handler error: %v", err)
	}
	c.JSON(status, gin.H{"error": publicMsg})
}

// UploadFiles handles POST /files (multipart form). Owner is read from gin context (set by auth middleware).
func (h *Handler) UploadFiles(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()
	log.Printf("UploadFiles start from %s", c.ClientIP())

	form, err := c.MultipartForm()
	if err != nil {
		log.Printf("UploadFiles: parse multipart error: %v, elapsed=%s", err, time.Since(start))
		h.respondError(c, 400, "failed to parse multipart form", err)
		return
	}
	log.Printf("UploadFiles: parsed multipart, elapsed=%s", time.Since(start))

	headers := form.File["files"]
	if len(headers) == 0 {
		c.JSON(400, gin.H{"error": "no files provided under form field 'files'"})
		return
	}

	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	resp, err := h.filesUC.UploadMultipart(ctx, ownerID, headers)
	if err != nil {
		log.Printf("UploadFiles: save error: %v, elapsed since start=%s", err, time.Since(start))
		h.respondError(c, 500, "failed to save file", err)
		return
	}

	log.Printf("UploadFiles: completed %d files, total elapsed=%s", len(headers), time.Since(start))
	c.JSON(200, resp)
}

// GetFile handles GET /files/:id and returns the file. Owner is verified from gin context.
func (h *Handler) GetFile(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "missing file id"})
		return
	}

	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	meta, b, err := h.filesUC.GetForOwner(ctx, ownerID, id)
	if err != nil {
		switch err {
		case files.ErrForbidden:
			h.respondError(c, 403, "forbidden", err)
		case files.ErrNotFound:
			h.respondError(c, 404, "file not found", err)
		default:
			h.respondError(c, 500, "failed to get file", err)
		}
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", meta.Filename))
	c.Status(200)
	if _, err := c.Writer.Write(b); err != nil {
		// writing failed; cannot change headers now
		return
	}
}

// CreateUser handles POST /users (register)
func (h *Handler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	if h.authUC == nil {
		c.JSON(500, gin.H{"error": "auth service not configured"})
		return
	}
	var rq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&rq); err != nil {
		h.respondError(c, 400, "invalid body", err)
		return
	}

	id, err := h.authUC.Register(ctx, rq.Username, rq.Password)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			h.respondError(c, 400, "username and password required", err)
			return
		}
		h.respondError(c, 500, "failed to create user", err)
		return
	}

	c.JSON(201, gin.H{"id": id, "username": rq.Username})
}

// Login handles POST /login with same username/password used in registration and returns JWT
func (h *Handler) Login(c *gin.Context) {
	if h.authUC == nil {
		c.JSON(500, gin.H{"error": "auth service not configured"})
		return
	}
	var rq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&rq); err != nil {
		h.respondError(c, 400, "invalid body", err)
		return
	}
	okTok, err := h.authUC.Login(c.Request.Context(), rq.Username, rq.Password)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			h.respondError(c, 400, "username and password required", err)
			return
		}
		h.respondError(c, 401, "invalid credentials", err)
		return
	}
	c.JSON(200, gin.H{"token": okTok})
}

// ListFiles returns JSON list of files for authenticated owner.
func (h *Handler) ListFiles(c *gin.Context) {
	ctx := c.Request.Context()
	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	files, err := h.filesUC.ListByOwner(ctx, ownerID)
	if err != nil {
		h.respondError(c, 500, "failed to list files", err)
		return
	}

	c.JSON(200, files)
}

// DeleteFile handles DELETE /files/:id
func (h *Handler) DeleteFile(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "missing file id"})
		return
	}

	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	_, err := h.filesUC.DeleteForOwner(ctx, ownerID, id)
	if err != nil {
		switch err {
		case files.ErrForbidden:
			h.respondError(c, 403, "forbidden", err)
		case files.ErrNotFound:
			h.respondError(c, 404, "file not found", err)
		default:
			h.respondError(c, 500, "failed to delete file", err)
		}
		return
	}

	c.Status(204)
}

// ListSharedFiles returns files shared with the authenticated user.
func (h *Handler) ListSharedFiles(c *gin.Context) {
	ctx := c.Request.Context()
	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	files, err := h.filesUC.ListShared(ctx, ownerID)
	if err != nil {
		h.respondError(c, 500, "failed to list shared files", err)
		return
	}

	c.JSON(200, files)
}

// GetSharedFile handles GET /files/shared/:id
func (h *Handler) GetSharedFile(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "missing file id"})
		return
	}

	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	meta, b, err := h.filesUC.GetForUser(ctx, ownerID, id)
	if err != nil {
		switch err {
		case files.ErrForbidden:
			h.respondError(c, 403, "forbidden", err)
		case files.ErrNotFound:
			h.respondError(c, 404, "file not found", err)
		default:
			h.respondError(c, 500, "failed to get file", err)
		}
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", meta.Filename))
	c.Status(200)
	if _, err := c.Writer.Write(b); err != nil {
		return
	}
}

// ShareFile grants access to a file by username.
func (h *Handler) ShareFile(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "missing file id"})
		return
	}

	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	var rq struct {
		Username string `json:"username"`
	}
	if err := c.BindJSON(&rq); err != nil {
		h.respondError(c, 400, "invalid body", err)
		return
	}

	if err := h.filesUC.ShareByUsername(ctx, ownerID, id, rq.Username, fileaccess.OwnerTGInfo{}); err != nil {
		switch err {
		case files.ErrForbidden:
			h.respondError(c, 403, "forbidden", err)
		case usecase.ErrUserNotFound:
			h.respondError(c, 404, "user not found", err)
		case files.ErrNotFound:
			h.respondError(c, 404, "file not found", err)
		default:
			h.respondError(c, 500, "failed to share file", err)
		}
		return
	}

	c.Status(204)
}

// ListGrantedUsers returns users that have access to a file.
func (h *Handler) ListGrantedUsers(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "missing file id"})
		return
	}
	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	users, err := h.filesUC.ListGrantedUsers(ctx, ownerID, id)
	if err != nil {
		switch err {
		case files.ErrForbidden:
			h.respondError(c, 403, "forbidden", err)
		case files.ErrNotFound:
			h.respondError(c, 404, "file not found", err)
		default:
			h.respondError(c, 500, "failed to list granted users", err)
		}
		return
	}

	c.JSON(200, users)
}

// RevokeShare revokes access for a user by username.
func (h *Handler) RevokeShare(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	username := c.Param("username")
	if id == "" || username == "" {
		c.JSON(400, gin.H{"error": "missing file id or username"})
		return
	}
	ownerVal, ok := c.Get("ownerID")
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found in context"})
		return
	}
	ownerID, ok := ownerVal.(string)
	if !ok || ownerID == "" {
		c.JSON(401, gin.H{"error": "unauthorized: invalid owner id"})
		return
	}

	if err := h.filesUC.RevokeShareByUsername(ctx, ownerID, id, username); err != nil {
		switch err {
		case files.ErrForbidden:
			h.respondError(c, 403, "forbidden", err)
		case files.ErrNotFound:
			h.respondError(c, 404, "file not found", err)
		case usecase.ErrUserNotFound:
			h.respondError(c, 404, "user not found", err)
		default:
			h.respondError(c, 500, "failed to revoke share", err)
		}
		return
	}
	c.Status(204)
}
