package fakes

import (
	"context"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// This file provides "always fail" doubles for every outbound port.
// They exist so use-case tests can prove the *unexpected
// infrastructure failure* path — the one that must surface a wrapped
// error AND an ERROR-level log line. The happy-path and
// domain-sentinel paths are covered by the stateful fakes elsewhere
// in this package.
//
// Convention: a single configurable error, defaulting to a
// recognisable infrastructure-style error so a forgetful test still
// sees a non-nil failure.

// ErrInjected is the default fault returned by the failing doubles.
var ErrInjected = &injectedError{}

type injectedError struct{}

func (*injectedError) Error() string { return "fakes: injected infrastructure failure" }

// ─── User ────────────────────────────────────────────────────────────

// FailingUserRepo fails every user-repository call (unexpected-infra path).
type FailingUserRepo struct{ Err error }

func (f FailingUserRepo) err() error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

var (
	_ user.Reader        = FailingUserRepo{}
	_ user.Writer        = FailingUserRepo{}
	_ user.StatusUpdater = FailingUserRepo{}
	_ user.AvatarUpdater = FailingUserRepo{}
)

func (f FailingUserRepo) Save(context.Context, user.User) (user.User, error) {
	return user.User{}, f.err()
}
func (f FailingUserRepo) FindByID(context.Context, shared.ID) (user.User, error) {
	return user.User{}, f.err()
}
func (f FailingUserRepo) FindByUsername(context.Context, user.Username) (user.User, error) {
	return user.User{}, f.err()
}
func (f FailingUserRepo) UpdateStatus(context.Context, shared.ID, user.Status) error { return f.err() }
func (f FailingUserRepo) UpdateAvatar(context.Context, shared.ID, string) error      { return f.err() }

// ─── Brotherband ─────────────────────────────────────────────────────

// FailingBrotherhoodRepo fails every brotherhood call.
type FailingBrotherhoodRepo struct{ Err error }

func (f FailingBrotherhoodRepo) err() error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

var _ brotherband.BrotherhoodRepository = FailingBrotherhoodRepo{}

func (f FailingBrotherhoodRepo) Save(context.Context, brotherband.Brotherhood) error { return f.err() }
func (f FailingBrotherhoodRepo) Delete(context.Context, shared.ID, shared.ID) error  { return f.err() }
func (f FailingBrotherhoodRepo) Exists(context.Context, shared.ID, shared.ID) (bool, error) {
	return false, f.err()
}
func (f FailingBrotherhoodRepo) ListBrothers(context.Context, shared.ID) ([]brotherband.Brother, error) {
	return nil, f.err()
}

// FailingRequestRepo fails every request-repository call.
type FailingRequestRepo struct{ Err error }

func (f FailingRequestRepo) err() error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

var _ brotherband.RequestRepository = FailingRequestRepo{}

func (f FailingRequestRepo) Save(context.Context, brotherband.Request) (brotherband.Request, error) {
	return brotherband.Request{}, f.err()
}
func (f FailingRequestRepo) FindByID(context.Context, shared.ID) (brotherband.Request, error) {
	return brotherband.Request{}, f.err()
}
func (f FailingRequestRepo) Delete(context.Context, shared.ID) error { return f.err() }
func (f FailingRequestRepo) ListReceived(context.Context, shared.ID) ([]brotherband.ReceivedRequest, error) {
	return nil, f.err()
}
func (f FailingRequestRepo) ListSent(context.Context, shared.ID) ([]brotherband.SentRequest, error) {
	return nil, f.err()
}

// ─── Message ─────────────────────────────────────────────────────────

// FailingMessageRepo fails every message-repository call.
type FailingMessageRepo struct{ Err error }

func (f FailingMessageRepo) err() error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

var (
	_ message.MessageReader = FailingMessageRepo{}
	_ message.MessageWriter = FailingMessageRepo{}
)

func (f FailingMessageRepo) Save(context.Context, message.Message) (message.Message, error) {
	return message.Message{}, f.err()
}
func (f FailingMessageRepo) FindByID(context.Context, shared.ID) (message.Message, error) {
	return message.Message{}, f.err()
}
func (f FailingMessageRepo) SaveAttachment(context.Context, message.Attachment) (message.Attachment, error) {
	return message.Attachment{}, f.err()
}
func (f FailingMessageRepo) FindByConversation(context.Context, shared.ID, *message.Cursor, int) ([]message.Message, *message.Cursor, error) {
	return nil, nil, f.err()
}

// FailingConversationRepo fails every conversation call.
type FailingConversationRepo struct{ Err error }

func (f FailingConversationRepo) err() error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

var _ message.ConversationRepository = FailingConversationRepo{}

func (f FailingConversationRepo) Create(context.Context, message.Conversation, []shared.ID) (message.Conversation, error) {
	return message.Conversation{}, f.err()
}
func (f FailingConversationRepo) FindDirectBetween(context.Context, shared.ID, shared.ID) (message.Conversation, bool, error) {
	return message.Conversation{}, false, f.err()
}
func (f FailingConversationRepo) IsParticipant(context.Context, shared.ID, shared.ID) (bool, error) {
	return false, f.err()
}
func (f FailingConversationRepo) UpdateLastRead(context.Context, shared.ID, shared.ID, time.Time) error {
	return f.err()
}

// ─── Media / auth ports ──────────────────────────────────────────────

// FailingImageStore fails every image-store call.
type FailingImageStore struct{ Err error }

func (f FailingImageStore) err() error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

var _ media.ImageStore = FailingImageStore{}

func (f FailingImageStore) PresignUpload(context.Context, shared.ID, media.AllowedContentType, int64) (media.PresignedUpload, error) {
	return media.PresignedUpload{}, f.err()
}
func (f FailingImageStore) PublicURL(string) string { return "" }
func (f FailingImageStore) PromoteFromPending(context.Context, string, string) error {
	return f.err()
}

// FailingHasher fails Hash and Verify.
type FailingHasher struct{ Err error }

var _ port.PasswordHasher = FailingHasher{}

func (f FailingHasher) Hash(string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return "", ErrInjected
}
func (f FailingHasher) Verify(string, string) error {
	if f.Err != nil {
		return f.Err
	}
	return ErrInjected
}

// FailingTokenIssuer fails Issue and Verify.
type FailingTokenIssuer struct{ Err error }

var _ port.TokenIssuer = FailingTokenIssuer{}

func (f FailingTokenIssuer) Issue(shared.ID, time.Time) (port.IssuedToken, error) {
	if f.Err != nil {
		return port.IssuedToken{}, f.Err
	}
	return port.IssuedToken{}, ErrInjected
}
func (f FailingTokenIssuer) Verify(string, time.Time) (shared.ID, error) {
	if f.Err != nil {
		return shared.ID{}, f.Err
	}
	return shared.ID{}, ErrInjected
}

// StaticHasher is a deterministic, non-failing hasher for use-case
// tests that need the happy path without the argon2id cost. The
// "hash" is just "hash:" + plain so Verify is a string compare.
type StaticHasher struct{}

var _ port.PasswordHasher = StaticHasher{}

func (StaticHasher) Hash(plain string) (string, error) { return "hash:" + plain, nil }
func (StaticHasher) Verify(encoded, plain string) error {
	if encoded == "hash:"+plain {
		return nil
	}
	return user.ErrInvalidCredentials
}
