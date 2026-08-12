package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	filesApp "core-orchestrator/internal/application/files"
)

type FileHandler struct {
	service *filesApp.FileService
}

type upsertFileRequest struct {
	DiskID           int64   `json:"diskId"`
	Path             string  `json:"path"`
	Filename         string  `json:"filename"`
	OriginalFilename *string `json:"originalFilename"`
	MimeType         string  `json:"mimeType"`
	FileType         string  `json:"fileType"`
	Extension        string  `json:"extension"`
	Size             int64   `json:"size"`
	Checksum         *string `json:"checksum"`
	Width            *int64  `json:"width"`
	Height           *int64  `json:"height"`
	IsPublic         *bool   `json:"isPublic"`
}

func NewFileHandler(service *filesApp.FileService) *FileHandler {
	return &FileHandler{service: service}
}

func (h *FileHandler) GetFiles(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset := 0
	pageSize := 10

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	if pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsedPageSize
		}
	}

	result, err := h.service.GetPaginatedFiles(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *FileHandler) GetFileByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseFileID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	file, err := h.service.GetFileByID(id)
	if err != nil {
		if errors.Is(err, filesApp.ErrInvalidFilePayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid file id")
			return
		}
		if errors.Is(err, filesApp.ErrFileNotFound) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(file)
}

func (h *FileHandler) CreateFile(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	input, err := decodeMultipartFileRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	file, err := h.service.CreateFile(input, user.ID)
	if err != nil {
		if errors.Is(err, filesApp.ErrInvalidFilePayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid file payload")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(file)
}

func (h *FileHandler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseFileID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	input, err := decodeFileRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	file, err := h.service.UpdateFile(id, input, user.ID)
	if err != nil {
		if errors.Is(err, filesApp.ErrInvalidFilePayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid file payload")
			return
		}
		if errors.Is(err, filesApp.ErrFileNotFound) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(file)
}

func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseFileID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	err = h.service.SoftDeleteFile(id, user.ID)
	if err != nil {
		if errors.Is(err, filesApp.ErrInvalidFilePayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid file id")
			return
		}
		if errors.Is(err, filesApp.ErrFileNotFound) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseFileID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

func decodeFileRequest(r *http.Request) (filesApp.UpsertFileInput, error) {
	var body upsertFileRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return filesApp.UpsertFileInput{}, err
	}

	isPublic := true
	if body.IsPublic != nil {
		isPublic = *body.IsPublic
	}

	return filesApp.UpsertFileInput{
		DiskID:           body.DiskID,
		Path:             body.Path,
		Filename:         body.Filename,
		OriginalFilename: body.OriginalFilename,
		MimeType:         body.MimeType,
		FileType:         body.FileType,
		Extension:        body.Extension,
		Size:             body.Size,
		Checksum:         body.Checksum,
		Width:            body.Width,
		Height:           body.Height,
		IsPublic:         isPublic,
	}, nil
}

func decodeMultipartFileRequest(r *http.Request) (filesApp.UpsertFileInput, error) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return filesApp.UpsertFileInput{}, errors.New("invalid multipart payload")
	}

	diskID, err := parseRequiredInt64Field(r.FormValue("diskId"))
	if err != nil {
		return filesApp.UpsertFileInput{}, errors.New("diskId is required and must be a positive integer")
	}

	filePart, fileHeader, err := r.FormFile("file")
	if err != nil {
		return filesApp.UpsertFileInput{}, errors.New("file is required")
	}
	defer filePart.Close()

	fileBytes, err := ioReadAll(filePart)
	if err != nil {
		return filesApp.UpsertFileInput{}, errors.New("unable to read uploaded file")
	}

	size := int64(len(fileBytes))
	if size <= 0 {
		return filesApp.UpsertFileInput{}, errors.New("uploaded file is empty")
	}

	sum := sha256.Sum256(fileBytes)
	checksum := hex.EncodeToString(sum[:])

	path := strings.TrimSpace(r.FormValue("path"))
	if path == "" {
		path = "/"
	}

	filename := strings.TrimSpace(r.FormValue("filename"))
	if filename == "" {
		filename = strings.TrimSpace(fileHeader.Filename)
	}

	originalFilename := strings.TrimSpace(fileHeader.Filename)
	var originalFilenamePtr *string
	if originalFilename != "" {
		originalFilenamePtr = &originalFilename
	}

	mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = detectMimeType(fileBytes)
	}

	extension := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(r.FormValue("extension")), "."))
	if extension == "" {
		extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	}
	if extension == "" {
		extension = "bin"
	}

	fileType := strings.TrimSpace(strings.ToLower(r.FormValue("fileType")))
	if fileType == "" {
		fileType = inferFileType(mimeType, extension)
	}

	isPublic := true
	isPublicRaw := strings.TrimSpace(r.FormValue("isPublic"))
	if isPublicRaw != "" {
		parsedIsPublic, parseErr := strconv.ParseBool(isPublicRaw)
		if parseErr != nil {
			return filesApp.UpsertFileInput{}, errors.New("isPublic must be a boolean")
		}
		isPublic = parsedIsPublic
	}

	width, err := parseOptionalInt64Field(r.FormValue("width"))
	if err != nil {
		return filesApp.UpsertFileInput{}, errors.New("width must be a positive integer")
	}

	height, err := parseOptionalInt64Field(r.FormValue("height"))
	if err != nil {
		return filesApp.UpsertFileInput{}, errors.New("height must be a positive integer")
	}

	return filesApp.UpsertFileInput{
		DiskID:           diskID,
		Path:             path,
		Filename:         filename,
		OriginalFilename: originalFilenamePtr,
		MimeType:         mimeType,
		FileType:         fileType,
		Extension:        extension,
		Size:             size,
		Checksum:         &checksum,
		Width:            width,
		Height:           height,
		IsPublic:         isPublic,
	}, nil
}

func detectMimeType(content []byte) string {
	if len(content) == 0 {
		return "application/octet-stream"
	}

	sample := content
	if len(sample) > 512 {
		sample = sample[:512]
	}

	return http.DetectContentType(sample)
}

func inferFileType(mimeType, extension string) string {
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(extension), "."))

	if ext == "ico" || ext == "svg" {
		return "icon"
	}

	if strings.HasPrefix(lowerMime, "image/") {
		return "image"
	}

	if strings.HasPrefix(lowerMime, "video/") {
		return "video"
	}

	if strings.HasPrefix(lowerMime, "application/pdf") || strings.HasPrefix(lowerMime, "text/") {
		return "document"
	}

	if ext == "zip" || ext == "rar" || ext == "7z" || ext == "tar" || ext == "gz" {
		return "archive"
	}

	return "other"
}

func parseRequiredInt64Field(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("required")
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid")
	}

	return parsed, nil
}

func parseOptionalInt64Field(raw string) (*int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, errors.New("invalid")
	}

	return &parsed, nil
}

func ioReadAll(file multipartFile) ([]byte, error) {
	return io.ReadAll(file)
}

type multipartFile interface {
	Read(p []byte) (n int, err error)
}
