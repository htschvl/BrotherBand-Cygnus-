package handler

import (
	"net/http"
	"strconv"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/dto"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	usecasemsg "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/message"
)

// MessageHandler owns the message-side routes plus the conversations
// list and the attachment-attach endpoint.
type MessageHandler struct {
	send              *usecasemsg.SendMessage
	list              *usecasemsg.ListMessages
	attach            *usecasemsg.AttachMedia
	listConversations *usecasemsg.ListConversations
}

// NewMessageHandler wires the message + conversation use cases.
func NewMessageHandler(
	send *usecasemsg.SendMessage,
	list *usecasemsg.ListMessages,
	attach *usecasemsg.AttachMedia,
	listConversations *usecasemsg.ListConversations,
) *MessageHandler {
	return &MessageHandler{send: send, list: list, attach: attach, listConversations: listConversations}
}

func (h *MessageHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	views, err := h.listConversations.Execute(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	out := dto.ConversationListResponse{Conversations: make([]dto.ConversationSummaryResponse, 0, len(views))}
	for _, v := range views {
		out.Conversations = append(out.Conversations, dto.ConversationFromUseCase(v))
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	brotherID, err := paramID(r, "brotherId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, parseErr := strconv.Atoi(raw); parseErr == nil {
			limit = v
		}
	}
	out, err := h.list.Execute(r.Context(), usecasemsg.ListInput{
		ActorID:   middleware.UserIDFromContext(r.Context()),
		BrotherID: brotherID,
		Cursor:    r.URL.Query().Get("cursor"),
		Limit:     limit,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	resp := dto.MessagePageResponse{
		Items: make([]dto.MessageResponse, 0, len(out.Items)),
	}
	if out.NextCursor != "" {
		c := out.NextCursor
		resp.NextCursor = &c
	}
	for _, m := range out.Items {
		resp.Items = append(resp.Items, dto.MessageFromUseCase(m))
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	brotherID, err := paramID(r, "brotherId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	var req dto.SendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	out, err := h.send.Execute(r.Context(), usecasemsg.SendInput{
		SenderID:  middleware.UserIDFromContext(r.Context()),
		BrotherID: brotherID,
		Body:      req.Body,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, dto.MessageFromUseCase(out))
}

func (h *MessageHandler) Attach(w http.ResponseWriter, r *http.Request) {
	messageID, err := paramID(r, "messageId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	var req dto.AttachMediaRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	out, err := h.attach.Execute(r.Context(), usecasemsg.AttachInput{
		ActorID:   middleware.UserIDFromContext(r.Context()),
		MessageID: messageID,
		MediaKey:  req.MediaKey,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.MessageFromUseCase(out))
}
