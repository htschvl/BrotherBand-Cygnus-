package handler

import (
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/dto"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// AuthHandler owns the three lifecycle routes: register, login,
// logout. Two of them call into a use case; logout is a pure
// cookie-clearing operation.
type AuthHandler struct {
	register *usecaseuser.RegisterUser
	login    *usecaseuser.LoginUser
	cookies  respond.CookieConfig
}

// NewAuthHandler wires the auth handler with the register/login use cases and the resolved cookie attributes.
func NewAuthHandler(
	register *usecaseuser.RegisterUser,
	login *usecaseuser.LoginUser,
	cookies respond.CookieConfig,
) *AuthHandler {
	return &AuthHandler{register: register, login: login, cookies: cookies}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	session, err := h.register.Execute(r.Context(), req.ToUseCase())
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.WriteSession(w, h.cookies, session)
	respond.JSON(w, http.StatusCreated, dto.ProfileFromUseCase(session.Profile))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	session, err := h.login.Execute(r.Context(), req.ToUseCase())
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.WriteSession(w, h.cookies, session)
	respond.JSON(w, http.StatusOK, dto.ProfileFromUseCase(session.Profile))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, _ *http.Request) {
	respond.ClearSession(w, h.cookies)
	w.WriteHeader(http.StatusNoContent)
}
