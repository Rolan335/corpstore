package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"corpstore/internal/auth"
	"corpstore/internal/storage"
	"corpstore/internal/storage/meta"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store storage.Storage
	meta  meta.Store
	auth  *auth.Service
}

func NewHandler(s storage.Storage, m meta.Store) *Handler {
	return &Handler{store: s, meta: m}
}

type Store interface {
	Save(ctx context.Context, data []byte, filename string, ownerID string) (string, error)
	Get(ctx context.Context, id string) ([]byte, error)
	GetMetadata(ctx context.Context, id string) (string, string, error)
}

// SetAuth allows wiring auth.Service after Handler creation.
func (h *Handler) SetAuth(a *auth.Service) {
	h.auth = a
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

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(400, gin.H{"error": "no files provided under form field 'files'"})
		return
	}

	type fileResp struct {
		Filename string `json:"filename"`
		ID       string `json:"id"`
	}

	resp := make([]fileResp, 0, len(files))

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

	// validate owner exists (defensive)
	okExists, err := h.meta.UserExists(ctx, ownerID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to validate owner: " + err.Error()})
		return
	}
	if !okExists {
		c.JSON(401, gin.H{"error": "unauthorized: owner not found"})
		return
	}

	for i, fh := range files {
		fileStart := time.Now()
		log.Printf("UploadFiles: processing file %d name=%s", i, fh.Filename)

		f, err := fh.Open()
		if err != nil {
			log.Printf("UploadFiles: open file error: %v, elapsed=%s", err, time.Since(fileStart))
			c.JSON(500, gin.H{"error": "failed to open uploaded file: " + err.Error()})
			return
		}

		buf, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			log.Printf("UploadFiles: read file error: %v, elapsed=%s", err, time.Since(fileStart))
			c.JSON(500, gin.H{"error": "failed to read uploaded file: " + err.Error()})
			return
		}
		log.Printf("UploadFiles: read %d bytes for %s, elapsed=%s", len(buf), fh.Filename, time.Since(fileStart))

		// set per-file save timeout to avoid long blocking
		saveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		id, err := h.store.Save(saveCtx, buf, fh.Filename, ownerID)
		// cancel immediately to free timer resources
		cancel()
		if err != nil {
			log.Printf("UploadFiles: save error: %v, elapsed since start=%s", err, time.Since(start))
			c.JSON(500, gin.H{"error": "failed to save file: " + err.Error()})
			return
		}
		log.Printf("UploadFiles: saved file id=%s name=%s, elapsed since start=%s", id, fh.Filename, time.Since(start))

		resp = append(resp, fileResp{Filename: fh.Filename, ID: id})
	}

	log.Printf("UploadFiles: completed %d files, total elapsed=%s", len(files), time.Since(start))
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

	filename, fileOwner, err := h.store.GetMetadata(ctx, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "file metadata not found: " + err.Error()})
		return
	}

	// enforce owner match
	if fileOwner != ownerID {
		c.JSON(403, gin.H{"error": "forbidden: owner mismatch"})
		return
	}

	b, err := h.store.Get(ctx, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "file not found: " + err.Error()})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Status(200)
	if _, err := c.Writer.Write(b); err != nil {
		// writing failed; cannot change headers now
		return
	}
}

// CreateUser handles POST /users (register)
func (h *Handler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	var rq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&rq); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if rq.Username == "" || rq.Password == "" {
		c.JSON(400, gin.H{"error": "username and password required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(rq.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password: " + err.Error()})
		return
	}

	id, err := h.meta.CreateUser(ctx, rq.Username, string(hash))
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}

	c.JSON(201, gin.H{"id": id, "username": rq.Username})
}

// Login handles POST /login with same username/password used in registration and returns JWT
func (h *Handler) Login(c *gin.Context) {
	if h.auth == nil {
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
	if rq.Username == "" || rq.Password == "" {
		c.JSON(400, gin.H{"error": "username and password required"})
		return
	}
	okTok, err := h.auth.Login(c.Request.Context(), rq.Username, rq.Password)
	if err != nil {
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

	files, err := h.meta.ListFilesByOwner(ctx, ownerID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list files: " + err.Error()})
		return
	}

	c.JSON(200, files)
}
