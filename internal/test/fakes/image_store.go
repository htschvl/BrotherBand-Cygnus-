package fakes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// ImageStore is the in-memory media.ImageStore. It records the keys
// it was asked to sign and the promotions applied so tests can
// assert on storage behaviour without touching R2.
type ImageStore struct {
	mu        sync.Mutex
	BaseURL   string
	Promoted  map[string]string // pending → final
	Validity  time.Duration
	NextIndex int
}

// NewImageStore returns an in-memory image store that records promotions.
func NewImageStore() *ImageStore {
	return &ImageStore{BaseURL: "https://cdn.test.local", Promoted: map[string]string{}, Validity: 15 * time.Minute}
}

var _ media.ImageStore = (*ImageStore)(nil)

func (s *ImageStore) PresignUpload(_ context.Context, ownerID shared.ID, contentType media.AllowedContentType, contentLength int64) (media.PresignedUpload, error) {
	if contentLength <= 0 || contentLength > media.MaxUploadBytes {
		return media.PresignedUpload{}, media.ErrPayloadTooLarge
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NextIndex++
	key := fmt.Sprintf("pending/%s/upload-%d.bin", ownerID.String(), s.NextIndex)
	return media.PresignedUpload{
		UploadURL: "https://put.test.local/" + key,
		MediaKey:  key,
		ExpiresAt: time.Now().Add(s.Validity),
	}, nil
}

func (s *ImageStore) PublicURL(mediaKey string) string {
	return s.BaseURL + "/" + mediaKey
}

func (s *ImageStore) PromoteFromPending(_ context.Context, pendingKey, finalKey string) error {
	if pendingKey == "" || finalKey == "" {
		return media.ErrPromotionFailed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Promoted[pendingKey] = finalKey
	return nil
}
