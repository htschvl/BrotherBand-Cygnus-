package handler

import (
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/dto"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// UserHandler covers /v1/me and the two profile mutations.
type UserHandler struct {
	getProfile   *usecaseuser.GetProfile
	updateStatus *usecaseuser.UpdateStatus
	updateAvatar *usecaseuser.UpdateAvatar
}

// NewUserHandler wires the profile read + status/avatar mutation use cases.
func NewUserHandler(
	getProfile *usecaseuser.GetProfile,
	updateStatus *usecaseuser.UpdateStatus,
	updateAvatar *usecaseuser.UpdateAvatar,
) *UserHandler {
	return &UserHandler{getProfile: getProfile, updateStatus: updateStatus, updateAvatar: updateAvatar}
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	view, err := h.getProfile.Execute(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.ProfileFromUseCase(view))
}

func (h *UserHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	if err := h.updateStatus.Execute(r.Context(), usecaseuser.UpdateStatusInput{
		UserID: middleware.UserIDFromContext(r.Context()),
		Status: req.Status,
	}); err != nil {
		respond.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateAvatarRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	if err := h.updateAvatar.Execute(r.Context(), usecaseuser.UpdateAvatarInput{
		UserID:   middleware.UserIDFromContext(r.Context()),
		MediaKey: req.MediaKey,
	}); err != nil {
		respond.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
