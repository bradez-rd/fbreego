package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/service/media"
)

const maxMediaUploadRequestSize = media.MaxMediaFileSize*4/3 + (1 << 20)

func (s *Server) handleUploadMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaUploadRequestSize)
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(50 << 20) // 50MB
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				JSONError(w, "PAYLOAD_TOO_LARGE", "Media upload exceeds the allowed size.", http.StatusRequestEntityTooLarge, nil, "")
				return
			}
			JSONError(w, "INVALID_MULTIPART", "Failed to parse multipart form.", http.StatusBadRequest, nil, "")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			JSONError(w, "FILE_MISSING", "No file found in form data under 'file'.", http.StatusBadRequest, nil, "")
			return
		}
		defer file.Close()

		mimeType := header.Header.Get("Content-Type")
		rec, err := s.MediaStore.Store(header.Filename, mimeType, file)
		if err != nil {
			JSONError(w, "STORE_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
			return
		}
		JSONSuccess(w, "Media uploaded successfully.", rec.Public(), http.StatusCreated)
		return
	}

	// JSON payload con base64
	var input types.MediaUploadInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			JSONError(w, "PAYLOAD_TOO_LARGE", "Media upload exceeds the allowed size.", http.StatusRequestEntityTooLarge, nil, "")
			return
		}
		JSONError(w, "INVALID_REQUEST", "Invalid JSON payload.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(input.ContentBase64)
	if err != nil {
		JSONError(w, "INVALID_BASE64", "Content is not valid base64.", http.StatusBadRequest, nil, "")
		return
	}

	rec, err := s.MediaStore.Store(input.FileName, input.MimeType, bytes.NewReader(decoded))
	if err != nil {
		JSONError(w, "STORE_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	JSONSuccess(w, "Media uploaded successfully.", rec.Public(), http.StatusCreated)
}

func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	items := s.MediaStore.ListAll(50)
	JSONSuccess(w, "Media items retrieved.", items, http.StatusOK)
}

func (s *Server) handleGetMediaFile(w http.ResponseWriter, r *http.Request) {
	mediaID := chi.URLParam(r, "mediaId")
	info, err := s.MediaStore.GetInfo(mediaID)
	if err != nil {
		JSONError(w, "MEDIA_NOT_FOUND", "Media file not found.", http.StatusNotFound, nil, "")
		return
	}

	filePath, err := s.MediaStore.GetFilePath(mediaID)
	if err != nil {
		JSONError(w, "MEDIA_FILE_ERROR", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	if info.MimeType != "" {
		w.Header().Set("Content-Type", info.MimeType)
	}
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleGetMediaInfo(w http.ResponseWriter, r *http.Request) {
	mediaID := chi.URLParam(r, "mediaId")
	info, err := s.MediaStore.GetInfo(mediaID)
	if err != nil {
		JSONError(w, "MEDIA_NOT_FOUND", "Media file not found.", http.StatusNotFound, nil, "")
		return
	}

	JSONSuccess(w, "Media information retrieved.", info.Public(), http.StatusOK)
}

func (s *Server) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	mediaID := chi.URLParam(r, "mediaId")
	deleted, err := s.MediaStore.Delete(mediaID)
	if err != nil {
		JSONError(w, "MEDIA_DELETE_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}
	if !deleted {
		JSONError(w, "MEDIA_NOT_FOUND", "Media file not found.", http.StatusNotFound, nil, "")
		return
	}

	JSONSuccess(w, "Media deleted successfully.", map[string]string{"mediaId": mediaID}, http.StatusOK)
}
