package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"corpstore/internal/storage"
	"corpstore/internal/storage/meta"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store storage.Storage
	meta  meta.MetaStore
}

func NewHandler(s storage.Storage, m meta.MetaStore) *Handler {
	return &Handler{store: s, meta: m}
}

// UploadFiles handles POST /files. Expects multipart form with field "files" containing one or more files.
// Returns JSON array of objects {"filename":"...","id":"..."}.
func (h *Handler) UploadFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (up to 32 MB in memory; larger parts will be stored in temp files).
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files provided under form field 'files'", http.StatusBadRequest)
		return
	}

	type fileResp struct {
		Filename string `json:"filename"`
		ID       string `json:"id"`
	}

	resp := make([]fileResp, 0, len(files))

	// owner ID can be passed via header X-Owner-ID for now
	ownerID := r.Header.Get("X-Owner-ID")

	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			http.Error(w, "failed to open uploaded file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Read into memory first; implementations of storage.Save expect an io.ReadSeeker.
		buf, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			http.Error(w, "failed to read uploaded file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		rs := bytes.NewReader(buf) // implements io.ReadSeeker

		id, err := h.store.Save(ctx, rs, fh.Filename, ownerID)
		if err != nil {
			http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		resp = append(resp, fileResp{Filename: fh.Filename, ID: id})
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(resp); err != nil {
		// encoding error
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetFile handles GET /files/{id} - streams the file back.
func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Expect URL like /files/{id}
	id := strings.TrimPrefix(r.URL.Path, "/files/")
	if id == "" {
		http.Error(w, "missing file id", http.StatusBadRequest)
		return
	}

	// fetch metadata (filename, owner)
	filename, _, err := h.store.GetMetadata(ctx, id)
	if err != nil {
		http.Error(w, "file metadata not found: "+err.Error(), http.StatusNotFound)
		return
	}

	rc, err := h.store.Get(ctx, id)
	if err != nil {
		http.Error(w, "file not found: "+err.Error(), http.StatusNotFound)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	if _, err := io.Copy(w, rc); err != nil {
		// cannot write response
		return
	}
}

// CreateUser handles POST /users {"username":"..."}
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type req struct {
		Username string `json:"username"`
	}
	var rq req
	if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if rq.Username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	id, err := h.meta.CreateUser(ctx, rq.Username)
	if err != nil {
		http.Error(w, "failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "username": rq.Username})
}

// Gin adapter for UploadFiles - forwards to existing net/http handler.
func (h *Handler) UploadFilesGin(c *gin.Context) {
	// reuse existing handler implementation
	h.UploadFiles(c.Writer, c.Request)
}

// Gin adapter for GetFile - forwards to existing net/http handler.
func (h *Handler) GetFileGin(c *gin.Context) {
	// we can use param :id in route, but existing handler reads from URL path.
	// Ensure the request URL is set to /files/{id} so the underlying handler parses it.
	id := c.Param("id")
	// create a shallow copy of request with adjusted URL.Path
	r := c.Request
	if r != nil {
		r2 := *r
		r2.URL.Path = "/files/" + id
		// call with modified request
		h.GetFile(c.Writer, &r2)
		return
	}
	c.Status(http.StatusInternalServerError)
}

// Gin adapter for CreateUser
func (h *Handler) CreateUserGin(c *gin.Context) {
	h.CreateUser(c.Writer, c.Request)
}
