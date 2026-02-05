package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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

// UploadFiles handles POST /files (multipart form). Owner is read from gin context (set by auth middleware).
func (h *Handler) UploadFiles(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()
	log.Printf("UploadFiles start from %s", c.ClientIP())

	form, err := c.MultipartForm()
	if err != nil {
		log.Printf("UploadFiles: parse multipart error: %v, elapsed=%s", err, time.Since(start))
		c.JSON(400, gin.H{"error": "failed to parse multipart form: " + err.Error()})
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
		c.JSON(500, gin.H{"error": "failed to save file: " + err.Error()})
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
			c.JSON(403, gin.H{"error": "forbidden: owner mismatch"})
		case files.ErrNotFound:
			c.JSON(404, gin.H{"error": "file not found"})
		default:
			c.JSON(500, gin.H{"error": "failed to get file: " + err.Error()})
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
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	id, err := h.authUC.Register(ctx, rq.Username, rq.Password)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(500, gin.H{"error": "failed to create user: " + err.Error()})
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
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	okTok, err := h.authUC.Login(c.Request.Context(), rq.Username, rq.Password)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(401, gin.H{"error": "invalid credentials"})
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
		c.JSON(500, gin.H{"error": "failed to list files: " + err.Error()})
		return
	}

	c.JSON(200, files)
}
