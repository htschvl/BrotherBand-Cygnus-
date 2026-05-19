package handler

import (
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/dto"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	usecasemedia "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/media"
)

// MediaHandler owns the single presign endpoint. Reads bypass the
// API entirely (CDN-served), so this handler is the only place the
// API is on the upload path.
type MediaHandler struct {
	requestUpload *usecasemedia.RequestUpload
}

// NewMediaHandler wires the presign use case.
func NewMediaHandler(requestUpload *usecasemedia.RequestUpload) *MediaHandler {
	return &MediaHandler{requestUpload: requestUpload}
}

func (h *MediaHandler) RequestUploadURL(w http.ResponseWriter, r *http.Request) {
	var req dto.RequestUploadURLRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	out, err := h.requestUpload.Execute(r.Context(), usecasemedia.RequestUploadInput{
		OwnerID:       middleware.UserIDFromContext(r.Context()),
		ContentType:   req.ContentType,
		ContentLength: req.ContentLength,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.PresignedUploadFromUseCase(out))
}
