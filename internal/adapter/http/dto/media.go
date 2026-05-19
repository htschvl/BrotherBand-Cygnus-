package dto

import (
	"time"

	usecasemedia "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/media"
)

// RequestUploadURLRequest is the JSON body of POST /v1/media/upload-url.
type RequestUploadURLRequest struct {
	ContentType   string `json:"contentType"`
	ContentLength int64  `json:"contentLength"`
}

// PresignedUploadResponse is the presign result returned to the client.
type PresignedUploadResponse struct {
	UploadURL string    `json:"uploadUrl"`
	MediaKey  string    `json:"mediaKey"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// PresignedUploadFromUseCase maps the use-case output to the wire response.
func PresignedUploadFromUseCase(v usecasemedia.RequestUploadOutput) PresignedUploadResponse {
	return PresignedUploadResponse{
		UploadURL: v.UploadURL,
		MediaKey:  v.MediaKey,
		ExpiresAt: v.ExpiresAt,
	}
}
