package user

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// User is the User aggregate root. All construction goes through
// `New` (or `Rehydrate` for adapters loading existing state) so
// invariants cannot be violated from outside the package.
type User struct {
	id           shared.ID
	username     Username
	passwordHash PasswordHash
	birthdate    Birthdate
	secret       Secret
	status       Status
	favorites    Favorites
	avatarKey    *string
	registeredAt time.Time
}

// New constructs a brand-new user with a freshly generated ID and
// the registration timestamp set to `now`. All inputs are validated
// at the value-object boundary; any failure surfaces as a domain
// error.
func New(
	username Username,
	passwordHash PasswordHash,
	birthdate Birthdate,
	secret Secret,
	status Status,
	favorites Favorites,
	now time.Time,
) User {
	return User{
		id:           shared.NewID(),
		username:     username,
		passwordHash: passwordHash,
		birthdate:    birthdate,
		secret:       secret,
		status:       status,
		favorites:    favorites,
		registeredAt: now,
	}
}

// Rehydrate is the adapter-only constructor used to rebuild a user
// from persistent storage. It performs no validation because the
// invariants were enforced at write time.
func Rehydrate(
	id shared.ID,
	username Username,
	passwordHash PasswordHash,
	birthdate Birthdate,
	secret Secret,
	status Status,
	favorites Favorites,
	avatarKey *string,
	registeredAt time.Time,
) User {
	return User{
		id:           id,
		username:     username,
		passwordHash: passwordHash,
		birthdate:    birthdate,
		secret:       secret,
		status:       status,
		favorites:    favorites,
		avatarKey:    avatarKey,
		registeredAt: registeredAt,
	}
}

func (u User) ID() shared.ID              { return u.id }
func (u User) Username() Username         { return u.username }
func (u User) PasswordHash() PasswordHash { return u.passwordHash }
func (u User) Birthdate() Birthdate       { return u.birthdate }
func (u User) Secret() Secret             { return u.secret }
func (u User) Status() Status             { return u.status }
func (u User) Favorites() Favorites       { return u.favorites }
func (u User) AvatarKey() *string         { return u.avatarKey }
func (u User) RegisteredAt() time.Time    { return u.registeredAt }

// WithStatus returns a copy of the user with the status replaced.
// The user itself is immutable; callers persist the new value.
func (u User) WithStatus(s Status) User {
	u.status = s
	return u
}

// WithAvatarKey returns a copy with the avatar pointer replaced.
func (u User) WithAvatarKey(key string) User {
	u.avatarKey = &key
	return u
}

// ─── Username ──────────────────────────────────────────────────────────────

// Username is a normalised, validated handle.
type Username struct{ value string }

const (
	usernameMinLen = 3
	usernameMaxLen = 32
)

// FieldUsername is the canonical wire name for the username field —
// shared by the value-object error reasons and the HTTP DTO.
const (
	FieldUsername  = "username"
	FieldPassword  = "password"
	FieldBirthdate = "birthdate"
	FieldSecret    = "secret"
	FieldStatus    = "status"
	FieldFavorites = "favorites"
)

// NewUsername validates and normalises the input. Whitespace is
// trimmed, but case is preserved so the original capitalisation
// survives display lookups; equality is case-insensitive at the DB
// layer (CITEXT).
func NewUsername(raw string) (Username, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < usernameMinLen || len(trimmed) > usernameMaxLen {
		return Username{}, shared.WrapValidation(
			ErrInvalidUsername,
			FieldUsername,
			fmt.Sprintf("must be between %d and %d characters", usernameMinLen, usernameMaxLen),
		)
	}
	for _, r := range trimmed {
		if !isUsernameRune(r) {
			return Username{}, shared.WrapValidation(
				ErrInvalidUsername,
				FieldUsername,
				"may only contain letters, digits, '_' or '-'",
			)
		}
	}
	return Username{value: trimmed}, nil
}

func isUsernameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func (n Username) String() string { return n.value }

// ─── PasswordHash ──────────────────────────────────────────────────────────

// PasswordHash wraps the encoded argon2id string. Construction is
// deliberately lenient (any non-empty string), because the hash is
// produced by an adapter and the domain only stores it.
type PasswordHash struct{ value string }

// NewPasswordHash wraps a non-empty hash string.
func NewPasswordHash(encoded string) (PasswordHash, error) {
	if encoded == "" {
		return PasswordHash{}, ErrInvalidPasswordHash
	}
	return PasswordHash{value: encoded}, nil
}

func (p PasswordHash) String() string { return p.value }

// Password policy constants are exported so the HTTP DTO can render
// consistent client-side validation messages.
const (
	PasswordMinLen = 8
	PasswordMaxLen = 128
)

// ValidateRawPassword enforces the password policy applied at
// registration. It is exported on the package, not the value object,
// because the raw password never enters domain state.
func ValidateRawPassword(raw string) error {
	if len(raw) < PasswordMinLen || len(raw) > PasswordMaxLen {
		return shared.WrapValidation(
			ErrPasswordTooWeak,
			FieldPassword,
			fmt.Sprintf("must be between %d and %d characters", PasswordMinLen, PasswordMaxLen),
		)
	}
	return nil
}

// ─── Birthdate ─────────────────────────────────────────────────────────────

// Birthdate is a calendar-day value (no time component).
type Birthdate struct{ value time.Time }

// NewBirthdate parses a YYYY-MM-DD string into a day-precision time.
func NewBirthdate(raw string) (Birthdate, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return Birthdate{}, shared.WrapValidation(
			ErrInvalidBirthdate,
			FieldBirthdate,
			"must be a calendar date in YYYY-MM-DD format",
		)
	}
	return Birthdate{value: t}, nil
}

// FromTime constructs a Birthdate from a Time, truncating to day.
func BirthdateFromTime(t time.Time) Birthdate {
	return Birthdate{value: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

func (b Birthdate) Time() time.Time { return b.value }
func (b Birthdate) String() string  { return b.value.Format("2006-01-02") }

// ─── Secret ────────────────────────────────────────────────────────────────

// Secret is the personal phrase a user reveals to those who accept
// their brotherband request.
type Secret struct{ value string }

const (
	secretMinLen = 1
	secretMaxLen = 280
)

// NewSecret validates and trims the personal secret. It is the
// phrase revealed exactly once to a user who accepts this user's
// brotherband request.
func NewSecret(raw string) (Secret, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < secretMinLen || len(trimmed) > secretMaxLen {
		return Secret{}, shared.WrapValidation(
			ErrInvalidSecret,
			FieldSecret,
			fmt.Sprintf("must be between %d and %d characters after trimming", secretMinLen, secretMaxLen),
		)
	}
	return Secret{value: trimmed}, nil
}

func (s Secret) String() string { return s.value }

// ─── Status ────────────────────────────────────────────────────────────────

// Status is the short free-form text shown next to a user's name.
type Status struct{ value string }

const (
	statusMinLen = 1
	statusMaxLen = 280
)

// NewStatus validates and trims the short free-form status line.
func NewStatus(raw string) (Status, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < statusMinLen || len(trimmed) > statusMaxLen {
		return Status{}, shared.WrapValidation(
			ErrInvalidStatus,
			FieldStatus,
			fmt.Sprintf("must be between %d and %d characters after trimming", statusMinLen, statusMaxLen),
		)
	}
	return Status{value: trimmed}, nil
}

func (s Status) String() string { return s.value }

// ─── Favorites ─────────────────────────────────────────────────────────────

// Favorites is exactly five short free-form strings the user lists
// at registration. The cardinality is a product invariant (it is the
// "small circle" signal) so it is enforced here rather than by the DB
// alone.
type Favorites struct{ values []string }

const FavoritesRequiredCount = 5
const FavoriteMaxLen = 80

// NewFavorites enforces the "small circle" product invariant: the
// list must contain exactly FavoritesRequiredCount non-empty entries.
func NewFavorites(raw []string) (Favorites, error) {
	if len(raw) != FavoritesRequiredCount {
		return Favorites{}, shared.WrapValidation(
			ErrInvalidFavorites,
			FieldFavorites,
			fmt.Sprintf("must contain exactly %d entries (got %d)", FavoritesRequiredCount, len(raw)),
		)
	}
	cleaned := make([]string, FavoritesRequiredCount)
	for i, v := range raw {
		t := strings.TrimSpace(v)
		if t == "" {
			return Favorites{}, shared.WrapValidation(
				ErrInvalidFavorites,
				fmt.Sprintf("%s[%d]", FieldFavorites, i),
				"may not be empty",
			)
		}
		if len(t) > FavoriteMaxLen {
			return Favorites{}, shared.WrapValidation(
				ErrInvalidFavorites,
				fmt.Sprintf("%s[%d]", FieldFavorites, i),
				fmt.Sprintf("must be at most %d characters", FavoriteMaxLen),
			)
		}
		cleaned[i] = t
	}
	return Favorites{values: cleaned}, nil
}

// Values returns a defensive copy so the internal slice cannot be
// mutated by callers.
func (f Favorites) Values() []string {
	out := make([]string, len(f.values))
	copy(out, f.values)
	return out
}
