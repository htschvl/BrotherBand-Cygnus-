package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/dto"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	usecasebb "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/brotherband"
)

// BrotherbandHandler owns every route under /v1/brothers and
// /v1/brotherband-requests.
type BrotherbandHandler struct {
	send         *usecasebb.SendRequest
	accept       *usecasebb.AcceptRequest
	deny         *usecasebb.DenyRequest
	cut          *usecasebb.CutBrotherband
	listRequests *usecasebb.ListRequests
	listBrothers *usecasebb.ListBrothers
	getBrother   *usecasebb.GetBrother
}

// NewBrotherbandHandler wires every brotherband / brothers use case behind one resource handler.
func NewBrotherbandHandler(
	send *usecasebb.SendRequest,
	accept *usecasebb.AcceptRequest,
	deny *usecasebb.DenyRequest,
	cut *usecasebb.CutBrotherband,
	listRequests *usecasebb.ListRequests,
	listBrothers *usecasebb.ListBrothers,
	getBrother *usecasebb.GetBrother,
) *BrotherbandHandler {
	return &BrotherbandHandler{
		send: send, accept: accept, deny: deny, cut: cut,
		listRequests: listRequests, listBrothers: listBrothers, getBrother: getBrother,
	}
}

func (h *BrotherbandHandler) ListBrothers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	views, err := h.listBrothers.Execute(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	out := dto.BrothersListResponse{Brothers: make([]dto.BrotherSummaryResponse, 0, len(views))}
	for _, v := range views {
		out.Brothers = append(out.Brothers, dto.BrotherSummaryFromUseCase(v))
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *BrotherbandHandler) GetBrother(w http.ResponseWriter, r *http.Request) {
	brotherID, err := paramID(r, "brotherId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	view, err := h.getBrother.Execute(r.Context(), middleware.UserIDFromContext(r.Context()), brotherID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.BrotherProfileResponse{
		BrotherSummaryResponse: dto.BrotherSummaryFromUseCase(view),
		Favorites:              view.Favorites,
	})
}

func (h *BrotherbandHandler) Cut(w http.ResponseWriter, r *http.Request) {
	brotherID, err := paramID(r, "brotherId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	if err := h.cut.Execute(r.Context(), usecasebb.CutInput{
		UserID:    middleware.UserIDFromContext(r.Context()),
		BrotherID: brotherID,
	}); err != nil {
		respond.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BrotherbandHandler) ListRequests(w http.ResponseWriter, r *http.Request) {
	out, err := h.listRequests.Execute(r.Context(), usecasebb.ListRequestsInput{
		UserID:    middleware.UserIDFromContext(r.Context()),
		Direction: r.URL.Query().Get("direction"),
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	resp := dto.BrotherbandRequestListResponse{
		Received: make([]dto.BrotherbandRequestResponse, 0, len(out.Received)),
		Sent:     make([]dto.BrotherbandRequestResponse, 0, len(out.Sent)),
	}
	for _, v := range out.Received {
		resp.Received = append(resp.Received, dto.RequestViewToResponse(v))
	}
	for _, v := range out.Sent {
		resp.Sent = append(resp.Sent, dto.RequestViewToResponse(v))
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *BrotherbandHandler) Send(w http.ResponseWriter, r *http.Request) {
	recipientID, err := paramID(r, "recipientId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	out, err := h.send.Execute(r.Context(), usecasebb.SendRequestInput{
		RequesterID: middleware.UserIDFromContext(r.Context()),
		RecipientID: recipientID,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, dto.BrotherbandRequestResponse{
		ID:                out.ID,
		RequesterID:       out.RequesterID,
		RecipientID:       out.RecipientID,
		RequesterUsername: out.RequesterUsername,
		RecipientUsername: out.RecipientUsername,
		CreatedAt:         out.CreatedAt,
	})
}

func (h *BrotherbandHandler) Accept(w http.ResponseWriter, r *http.Request) {
	requestID, err := paramID(r, "requestId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	out, err := h.accept.Execute(r.Context(), usecasebb.AcceptInput{
		UserID:    middleware.UserIDFromContext(r.Context()),
		RequestID: requestID,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, dto.BrotherbandAcceptedResponse{
		Brother:         dto.BrotherSummaryFromUseCase(out.Brother),
		RequesterSecret: out.RequesterSecret,
	})
}

func (h *BrotherbandHandler) Deny(w http.ResponseWriter, r *http.Request) {
	requestID, err := paramID(r, "requestId")
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	if err := h.deny.Execute(r.Context(), usecasebb.DenyInput{
		UserID:    middleware.UserIDFromContext(r.Context()),
		RequestID: requestID,
	}); err != nil {
		respond.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// paramID parses a chi URL parameter as a typed shared.ID. Bad input
// is normalised to shared.ErrInvalidID so the error map handles it.
func paramID(r *http.Request, name string) (shared.ID, error) {
	return shared.ParseID(chi.URLParam(r, name))
}
