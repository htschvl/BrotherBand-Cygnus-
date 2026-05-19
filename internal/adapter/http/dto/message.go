package dto

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	usecasemsg "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/message"
)

// SendMessageRequest is the JSON body of POST .../messages.
type SendMessageRequest struct {
	Body string `json:"body"`
}

// AttachMediaRequest is the JSON body of PATCH /v1/messages/{id}/attachment.
type AttachMediaRequest struct {
	MediaKey string `json:"mediaKey"`
}

// MessageAttachmentResponse is one rendered attachment (CDN URL + metadata).
type MessageAttachmentResponse struct {
	MediaKey    string `json:"mediaKey"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// MessageResponse is a single message on the wire.
type MessageResponse struct {
	ID             shared.ID                   `json:"id"`
	ConversationID shared.ID                   `json:"conversationId"`
	SenderID       shared.ID                   `json:"senderId"`
	Body           string                      `json:"body"`
	CreatedAt      time.Time                   `json:"createdAt"`
	EditedAt       *time.Time                  `json:"editedAt,omitempty"`
	Attachments    []MessageAttachmentResponse `json:"attachments,omitempty"`
}

// MessagePageResponse is one cursor-paginated page of messages.
type MessagePageResponse struct {
	Items      []MessageResponse `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

// ConversationSummaryResponse is one row in the conversations list.
type ConversationSummaryResponse struct {
	ConversationID     shared.ID              `json:"conversationId"`
	Brother            BrotherSummaryResponse `json:"brother"`
	LastMessageAt      *time.Time             `json:"lastMessageAt"`
	LastMessagePreview *string                `json:"lastMessagePreview,omitempty"`
	UnreadCount        int                    `json:"unreadCount"`
}

// ConversationListResponse is the GET /v1/conversations payload.
type ConversationListResponse struct {
	Conversations []ConversationSummaryResponse `json:"conversations"`
}

// MessageFromUseCase maps a use-case MessageView to the wire response.
func MessageFromUseCase(v usecasemsg.MessageView) MessageResponse {
	out := MessageResponse{
		ID:             v.ID,
		ConversationID: v.ConversationID,
		SenderID:       v.SenderID,
		Body:           v.Body,
		CreatedAt:      v.CreatedAt,
		EditedAt:       v.EditedAt,
	}
	for _, a := range v.Attachments {
		out.Attachments = append(out.Attachments, MessageAttachmentResponse{
			MediaKey:    a.MediaKey,
			URL:         a.URL,
			ContentType: a.ContentType,
			SizeBytes:   a.SizeBytes,
		})
	}
	return out
}

// ConversationFromUseCase maps a use-case summary to the wire response.
func ConversationFromUseCase(v usecasemsg.ConversationSummary) ConversationSummaryResponse {
	out := ConversationSummaryResponse{
		ConversationID: v.ConversationID,
		Brother: BrotherSummaryResponse{
			ID:        v.BrotherID,
			Username:  v.BrotherUsername,
			Status:    v.BrotherStatus,
			AvatarURL: v.BrotherAvatarURL,
		},
		LastMessageAt:      v.LastMessageAt,
		LastMessagePreview: v.LastMessagePreview,
		UnreadCount:        v.UnreadCount,
	}
	if !v.BecameBrothersAt.IsZero() {
		t := v.BecameBrothersAt
		out.Brother.BecameBrothersAt = &t
	}
	return out
}
