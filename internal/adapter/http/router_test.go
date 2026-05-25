package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httplayer "github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/handler"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/observability"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	usecasebb "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/brotherband"
	usecasemedia "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/media"
	usecasemsg "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/message"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// harness wires the *real* router (full middleware stack) with use
// cases backed by in-memory fakes. This is the layer-3 HTTP test:
// routing, middleware order, CSRF, auth, error mapping, cookies and
// the request-id contract all execute exactly as in production.
type harness struct {
	srv    *httptest.Server
	tokens *fakes.TokenIssuer
	cap    *logging.Capture
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	users := fakes.NewUserRepo()
	reqs := fakes.NewRequestRepo()
	bonds := fakes.NewBrotherhoodRepo()
	convs := fakes.NewConversationRepo()
	msgs := fakes.NewMessageRepo()
	img := fakes.NewImageStore()
	hasher := fakes.StaticHasher{}
	tokens := fakes.NewTokenIssuer()
	csrf := fakes.NewCSRFMinter("csrf")
	clk := clock.System{}
	cap := logging.NewCapture(slog.LevelDebug)

	routes := httplayer.Routes{
		Auth: handler.NewAuthHandler(
			usecaseuser.NewRegisterUser(users, users, hasher, tokens, csrf, clk, img),
			usecaseuser.NewLoginUser(users, hasher, tokens, csrf, clk, img),
			respond.CookieConfig{Secure: false},
		),
		User: handler.NewUserHandler(
			usecaseuser.NewGetProfile(users, img),
			usecaseuser.NewUpdateStatus(users),
			usecaseuser.NewUpdateAvatar(users, img),
		),
		Brotherband: handler.NewBrotherbandHandler(
			usecasebb.NewSendRequest(reqs, bonds, users, clk),
			usecasebb.NewAcceptRequest(reqs, bonds, users, img, clk),
			usecasebb.NewDenyRequest(reqs),
			usecasebb.NewCutBrotherband(bonds),
			usecasebb.NewListRequests(reqs),
			usecasebb.NewListBrothers(bonds, img),
			usecasebb.NewGetBrother(bonds, users, img),
		),
		Message: handler.NewMessageHandler(
			usecasemsg.NewSendMessage(convs, msgs, bonds, clk, img),
			usecasemsg.NewListMessages(convs, msgs, bonds, img),
			usecasemsg.NewAttachMedia(msgs, img, clk),
			usecasemsg.NewListConversations(bonds, convs, img),
		),
		Media:  handler.NewMediaHandler(usecasemedia.NewRequestUpload(img)),
		Health: handler.NewHealthHandler(nil, "test"), // /readyz untested (needs a pool)
	}

	router := httplayer.NewRouter(httplayer.RouterConfig{
		Logger:         cap.Logger(),
		Metrics:        observability.NewMetrics(),
		AllowedOrigins: []string{"http://localhost:3333"},
		TokenIssuer:    tokens,
		Clock:          clk,
	}, routes)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &harness{srv: srv, tokens: tokens, cap: cap}
}

type apiResponse struct {
	status  int
	body    []byte
	cookies []*http.Cookie
	header  http.Header
}

func (r apiResponse) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		t.Fatalf("body is not JSON object: %q (%v)", r.body, err)
	}
	return m
}

// do performs a request. session/csrf are optional; when csrf is set
// it is sent both as the bb_csrf cookie and the X-CSRF-Token header
// (the double-submit contract).
func (h *harness) do(t *testing.T, method, path string, body any, session, csrf string) apiResponse {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if session != "" {
		req.AddCookie(&http.Cookie{Name: middleware.CookieSession, Value: session})
	}
	if csrf != "" {
		req.AddCookie(&http.Cookie{Name: middleware.CookieCSRF, Value: csrf})
		req.Header.Set(middleware.HeaderCSRF, csrf)
	}
	resp, err := h.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return apiResponse{status: resp.StatusCode, body: raw, cookies: resp.Cookies(), header: resp.Header}
}

func cookie(resp apiResponse, name string) string {
	for _, c := range resp.cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func registerBody(username string) map[string]any {
	return map[string]any{
		"username":  username,
		"password":  "Hunter2!Hunter2",
		"birthdate": "1994-02-02",
		"secret":    "the secret of " + username,
		"status":    "here",
		"favorites": []string{"a", "b", "c", "d", "e"},
	}
}

// register returns (session, csrf, userID).
func (h *harness) register(t *testing.T, username string) (string, string, string) {
	t.Helper()
	resp := h.do(t, http.MethodPost, "/v1/auth/register", registerBody(username), "", "")
	if resp.status != http.StatusCreated {
		t.Fatalf("register %s: status %d body %s", username, resp.status, resp.body)
	}
	session := cookie(resp, middleware.CookieSession)
	csrf := cookie(resp, middleware.CookieCSRF)
	if session == "" || csrf == "" {
		t.Fatalf("register must set both cookies; got session=%q csrf=%q", session, csrf)
	}
	id, _ := resp.json(t)["id"].(string)
	if id == "" {
		t.Fatal("register response must include the user id")
	}
	return session, csrf, id
}

// ─── Health ──────────────────────────────────────────────────────────

func TestRouter_Healthz_NoAuthNeeded(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/healthz", nil, "", "")
	if resp.status != http.StatusOK {
		t.Fatalf("healthz: status %d", resp.status)
	}
	if resp.header.Get(middleware.RequestIDHeader) == "" {
		t.Fatal("every response must carry X-Request-ID")
	}
}

// ─── Auth + cookies ──────────────────────────────────────────────────

func TestRouter_RegisterSetsSecureCookieContract(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/v1/auth/register", registerBody("alice"), "", "")
	if resp.status != http.StatusCreated {
		t.Fatalf("status %d body %s", resp.status, resp.body)
	}
	var sessionCookie *http.Cookie
	for _, c := range resp.cookies {
		if c.Name == middleware.CookieSession {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("bb_session cookie missing")
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("bb_session MUST be HttpOnly (XSS cannot read it)")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("bb_session must be SameSite=Lax")
	}
}

func TestRouter_RegisterValidationReturns422WithFieldDetails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bad := registerBody("bob")
	bad["password"] = "short"
	resp := h.do(t, http.MethodPost, "/v1/auth/register", bad, "", "")
	if resp.status != http.StatusUnprocessableEntity {
		t.Fatalf("weak password must be 422, got %d (%s)", resp.status, resp.body)
	}
	m := resp.json(t)
	if m["code"] != "user.password_too_weak" {
		t.Fatalf("unexpected code: %v", m["code"])
	}
	details, ok := m["details"].(map[string]any)
	if !ok || details["field"] != "password" {
		t.Fatalf("expected details.field=password, got %#v", m["details"])
	}
	if m["requestId"] == "" || m["requestId"] == nil {
		t.Fatal("error body must echo requestId")
	}
}

func TestRouter_DuplicateUsernameReturns409(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.register(t, "carol")
	resp := h.do(t, http.MethodPost, "/v1/auth/register", registerBody("carol"), "", "")
	if resp.status != http.StatusConflict {
		t.Fatalf("duplicate username must be 409, got %d", resp.status)
	}
	if resp.json(t)["code"] != "user.username_taken" {
		t.Fatalf("unexpected code: %s", resp.body)
	}
}

func TestRouter_LoginWrongPasswordIs401(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.register(t, "dave")
	resp := h.do(t, http.MethodPost, "/v1/auth/login",
		map[string]any{"username": "dave", "password": "WrongPassword1"}, "", "")
	if resp.status != http.StatusUnauthorized {
		t.Fatalf("bad password must be 401, got %d", resp.status)
	}
	if resp.json(t)["code"] != "user.invalid_credentials" {
		t.Fatalf("unexpected code: %s", resp.body)
	}
}

// ─── Auth gating ─────────────────────────────────────────────────────

func TestRouter_ProtectedRouteWithoutSessionIs401(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/v1/me", nil, "", "")
	if resp.status != http.StatusUnauthorized {
		t.Fatalf("no session must be 401, got %d", resp.status)
	}
	if resp.json(t)["requestId"] == nil {
		t.Fatal("401 body must include requestId")
	}
}

func TestRouter_StateChangingWithoutCSRFIs403(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	session, _, _ := h.register(t, "erin")
	// Send the session but NOT the csrf header/cookie.
	resp := h.do(t, http.MethodPatch, "/v1/me/status", map[string]any{"status": "x"}, session, "")
	if resp.status != http.StatusForbidden {
		t.Fatalf("missing CSRF must be 403, got %d", resp.status)
	}
	if resp.json(t)["code"] != "csrf.mismatch" {
		t.Fatalf("unexpected code: %s", resp.body)
	}
}

func TestRouter_UnknownRouteIs404(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/v1/does-not-exist", nil, "", "")
	if resp.status != http.StatusNotFound {
		t.Fatalf("unknown route must be 404, got %d", resp.status)
	}
}

// ─── End-to-end product flow ─────────────────────────────────────────

func TestRouter_FullBrotherbandAndMessagingFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	aSession, aCSRF, aID := h.register(t, "anna")
	bSession, bCSRF, bID := h.register(t, "ben")

	// anna → ben brotherband request
	sendResp := h.do(t, http.MethodPost, "/v1/brotherband-requests/send/"+bID, nil, aSession, aCSRF)
	if sendResp.status != http.StatusCreated {
		t.Fatalf("send request: %d %s", sendResp.status, sendResp.body)
	}
	requestID, _ := sendResp.json(t)["id"].(string)
	if requestID == "" {
		t.Fatal("send response must return the request id")
	}

	// ben accepts → must receive anna's secret exactly once
	acceptResp := h.do(t, http.MethodPost, "/v1/brotherband-requests/"+requestID+"/accept", nil, bSession, bCSRF)
	if acceptResp.status != http.StatusCreated {
		t.Fatalf("accept: %d %s", acceptResp.status, acceptResp.body)
	}
	if secret, _ := acceptResp.json(t)["requesterSecret"].(string); secret != "the secret of anna" {
		t.Fatalf("secret reveal broken, got %q", secret)
	}

	// anna lists brothers → ben present
	bros := h.do(t, http.MethodGet, "/v1/brothers", nil, aSession, aCSRF)
	if bros.status != http.StatusOK {
		t.Fatalf("list brothers: %d %s", bros.status, bros.body)
	}
	if !strings.Contains(string(bros.body), bID) {
		t.Fatalf("ben should be in anna's brothers: %s", bros.body)
	}

	// anna messages ben
	msgResp := h.do(t, http.MethodPost, "/v1/conversations/with/"+bID+"/messages",
		map[string]any{"body": "hey ben"}, aSession, aCSRF)
	if msgResp.status != http.StatusCreated {
		t.Fatalf("send message: %d %s", msgResp.status, msgResp.body)
	}

	// ben lists the conversation messages
	listResp := h.do(t, http.MethodGet, "/v1/conversations/with/"+aID+"/messages", nil, bSession, bCSRF)
	if listResp.status != http.StatusOK {
		t.Fatalf("list messages: %d %s", listResp.status, listResp.body)
	}
	if !strings.Contains(string(listResp.body), "hey ben") {
		t.Fatalf("ben should see anna's message: %s", listResp.body)
	}
}

func TestRouter_MessagingANonBrotherIsForbidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	aSession, aCSRF, _ := h.register(t, "frank")
	_, _, strangerID := h.register(t, "grace")

	resp := h.do(t, http.MethodPost, "/v1/conversations/with/"+strangerID+"/messages",
		map[string]any{"body": "hi stranger"}, aSession, aCSRF)
	if resp.status != http.StatusForbidden {
		t.Fatalf("messaging a non-brother must be 403, got %d (%s)", resp.status, resp.body)
	}
	if resp.json(t)["code"] != "brotherband.not_a_brother" {
		t.Fatalf("unexpected code: %s", resp.body)
	}
}

func TestRouter_BadPathParamReturns422(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	session, csrf, _ := h.register(t, "heidi")
	resp := h.do(t, http.MethodPost, "/v1/brotherband-requests/send/not-a-uuid", nil, session, csrf)
	if resp.status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid uuid path param must be 422, got %d (%s)", resp.status, resp.body)
	}
}
