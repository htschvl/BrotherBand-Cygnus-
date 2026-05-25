# Brotherband / Cygnus — Backend Architecture (v3)

**Stack summary:** `net/http` + **Chi**, spec-first OpenAPI, `sqlc` + `pgx/v5` for Postgres
(single store, including messages), **goose** for schema migrations, stateless 30-day JWT
in httpOnly cookies for auth, **argon2id** for password hashing, **Cloudflare R2** for
image storage via presigned URLs with a size-bound signature, **Svelte 5** + TypeScript
frontend with `@hey-api/openapi-ts` for type-safe client generation, structured logging
via `slog` with request IDs, a Prometheus `/metrics` endpoint, and a four-layer test
strategy (unit → repo → HTTP handler → E2E) using `testing`, `httptest`, and Testcontainers.

---

## 1. Architecture — Clean layer map (committed)

The Dependency Rule: source-code dependencies point inward only. `infrastructure/` may
import `adapter/`, `usecase/`, and `domain/`. `adapter/` may import `usecase/` and
`domain/`. `usecase/` may import `domain/` and `usecase/port/` only. `domain/` imports
nothing from this module. `cmd/api/main.go` is the composition root and is the only
file that touches every layer; it wires the graph.

```
.
├── api/
│   └── openapi.yaml                       # spec — source of truth
├── cmd/
│   └── api/
│       └── main.go                        # composition root
├── internal/
│   ├── domain/                            # innermost; zero external deps; pure Go
│   │   ├── shared/
│   │   │   ├── id.go                      # ID value type
│   │   │   └── errors.go                  # base error sentinels
│   │   ├── user/
│   │   │   ├── user.go                    # entity + value objects (Email, PasswordHash)
│   │   │   ├── repository.go              # Reader / Writer / AvatarUpdater — split per ISP
│   │   │   └── errors.go
│   │   ├── message/
│   │   │   ├── message.go                 # Message entity + invariants
│   │   │   ├── conversation.go            # Conversation + participant rules
│   │   │   ├── cursor.go                  # pagination cursor value object
│   │   │   ├── repository.go              # MessageReader / MessageWriter / ConversationRepo
│   │   │   └── errors.go
│   │   └── media/
│   │       ├── image_store.go             # ImageStore port
│   │       └── errors.go
│   │
│   ├── usecase/                           # application layer
│   │   ├── port/                          # cross-cutting ports
│   │   │   ├── password_hasher.go
│   │   │   ├── token_issuer.go
│   │   │   └── clock.go
│   │   ├── user/
│   │   │   ├── register_user.go           # one struct per use case
│   │   │   ├── login_user.go
│   │   │   ├── get_profile.go
│   │   │   ├── update_avatar.go
│   │   │   └── dto.go                     # Input/Output structs
│   │   ├── message/
│   │   │   ├── send_message.go
│   │   │   ├── list_messages.go
│   │   │   ├── attach_media.go
│   │   │   └── dto.go
│   │   └── media/
│   │       ├── request_upload.go
│   │       ├── promote_pending.go
│   │       └── dto.go
│   │
│   ├── adapter/                           # interface adapters: bridge domain ↔ external
│   │   ├── http/                          # inbound (driving) adapter
│   │   │   ├── server.go                  # http.Server config + lifecycle
│   │   │   ├── router.go                  # Chi route assembly only
│   │   │   ├── handler/
│   │   │   │   ├── user_handler.go        # one thin struct per resource
│   │   │   │   ├── message_handler.go
│   │   │   │   └── media_handler.go
│   │   │   ├── dto/                       # HTTP-shaped DTOs (separate from usecase DTOs)
│   │   │   │   ├── user.go
│   │   │   │   ├── message.go
│   │   │   │   └── media.go
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go                # bb_session cookie → user ID in context
│   │   │   │   ├── csrf.go
│   │   │   │   ├── request_id.go
│   │   │   │   ├── access_log.go
│   │   │   │   ├── rate_limit.go
│   │   │   │   └── recover.go
│   │   │   └── errormap.go                # domain.Err* → HTTP status mapping
│   │   ├── persistence/                   # outbound (driven) adapter
│   │   │   └── postgres/
│   │   │       ├── query/                 # sqlc input
│   │   │       │   ├── user.sql
│   │   │       │   ├── message.sql
│   │   │       │   └── conversation.sql
│   │   │       ├── sqlc.yaml
│   │   │       ├── generated/             # sqlc output — do not edit
│   │   │       ├── user_repository.go     # implements domain/user interfaces
│   │   │       ├── message_repository.go  # implements domain/message interfaces
│   │   │       └── mapper.go              # generated ↔ domain conversions
│   │   ├── auth/                          # outbound adapter for usecase/port interfaces
│   │   │   ├── jwt_issuer.go              # implements port.TokenIssuer
│   │   │   └── argon2id_hasher.go         # implements port.PasswordHasher
│   │   └── storage/
│   │       └── r2/
│   │           └── presigner.go           # implements domain/media.ImageStore
│   │
│   ├── infrastructure/                    # frameworks & drivers
│   │   ├── config/
│   │   │   └── config.go                  # env loading + validation
│   │   ├── postgres/
│   │   │   ├── pool.go                    # pgx pool factory
│   │   │   ├── migrate.go                 # goose runner
│   │   │   └── migrations/
│   │   │       └── 00001_init.sql
│   │   ├── s3/
│   │   │   └── client.go                  # aws-sdk-go-v2 client configured for R2
│   │   └── observability/
│   │       ├── logger.go                  # slog JSON handler factory
│   │       └── metrics.go                 # prometheus registry + collectors
│   │
│   ├── platform/                          # cross-cutting non-business helpers
│   │   └── clock/
│   │       └── system_clock.go            # implements port.Clock
│   │
│   └── test/                              # test helpers (internal, not importable outside module)
│       ├── fixtures/
│       │   └── user.go                    # builder-pattern fixtures
│       ├── fakes/                         # hand-rolled fakes for domain interfaces
│       │   ├── user_repo.go
│       │   └── clock.go
│       ├── mocks/                         # mockery-generated mocks for use-case interfaces
│       │   ├── register_user.go
│       │   └── login_user.go
│       └── containers/                    # Testcontainers session-scoped helpers
│           ├── postgres.go
│           └── minio.go
│
├── web/                                   # SvelteKit frontend (separate build/CI)
├── scripts/
│   ├── dev-up.sh                          # local pg + minio via docker-compose
│   └── generate.sh                        # sqlc + oapi-codegen + openapi-ts
├── Makefile
├── go.mod
├── go.sum
└── README.md
└── docs/                                  # General Documentation
    ├── brotherband-cygnus-doc-architecture.md
    ├── brotherband-cygnus-doc-biz.md
    └── brotherband-cygnus-doc-tech.md


```

### Why this layout (the parts that aren't obvious)

**Aggregate-scoped `domain/<aggregate>/`** (SRP at the package level). The User aggregate
owns its entity, value objects, repository interfaces, and errors in one place; the same
for Message and Media. `domain/shared/` holds genuinely cross-cutting bits (the `ID`
value type, base error sentinels) so aggregates never import each other and import
cycles can't form.

**`internal/usecase/port/` for cross-cutting interfaces** like `PasswordHasher`, `TokenIssuer`,
`Clock`. They're not domain concepts (a user aggregate doesn't care how passwords are
hashed) and not framework concerns (the use case defines the contract). They're use-case
ports. Putting them here makes the Dependency Inversion explicit: the inner layer
declares the interface, the outer layer (`adapter/auth/`, `platform/clock/`)
implements it.

**One use case = one struct** (strict Clean). `RegisterUser`, `LoginUser`, `UpdateAvatar`
are independent types with a single `Execute(ctx, Input) (Output, error)` method. This
is the SRP-maximal form. The pragmatic alternative — `UserService` with `Register`,
`Login`, `UpdateAvatar` methods — saves files but bundles unrelated responsibilities
and forces handlers to depend on the full service surface even when they use one method.

**Input/Output DTOs at the use case boundary** (in `usecase/<aggregate>/dto.go`). Use
cases don't expose domain types to handlers and don't accept HTTP types as input. This
keeps the use case insulated from both directions.

**HTTP DTOs separate from use-case DTOs.** Two adapters, two shapes: handler reads
JSON → HTTP DTO → use-case Input; use case returns Output → handler converts to HTTP
DTO → JSON. Annoying to type, but it's what prevents JSON tags and snake_case from
bleeding into use cases.

**`adapter/persistence/` instead of `adapter/db/`.** `db/` is a tech label;
`persistence/` is a Clean role. If a cache or search index adapter is added later, it
slots beside `postgres/` under the same umbrella.

**`adapter/auth/` exists as a peer of `adapter/persistence/`.** JWT issuance and
argon2id hashing implement use-case ports — they're adapters by definition.
`infrastructure/` is reserved for code that implements nothing inward-facing:
pool factories, migration runners, AWS SDK setup, slog handler factory, Prometheus
registry. The rule: if it implements a domain or use-case interface, it's in
`adapter/`; if it's pure technical wiring, it's in `infrastructure/`.

**`internal/test/`** is the only Go-idiomatic place to put test helpers that aren't
tied to one package. Calling it `testing/` would shadow the stdlib import name.

Chi is the router. It's a pure `net/http` wrapper: handlers are `http.HandlerFunc`,
middleware is `func(http.Handler) http.Handler`. The router lives entirely inside
`internal/adapter/http/`; domain and use-case packages import zero Chi symbols. Echo is
excluded mostly for surface area — its own `Context`, its own middleware contract —
not because adapting it is impossible.

---

## 2. Postgres Access — sqlc + pgx/v5 (committed)

**Choice: `sqlc` (v1.31+) + `pgx/v5`. No ORM.**

`sqlc` generates a `Querier` interface and a `DBTX` interface — both are critical for
the testing strategy in §11.

```yaml
# internal/adapter/persistence/postgres/sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "query/"
    schema: "../../../infrastructure/postgres/migrations/"
    gen:
      go:
        package: "db"
        out: "generated"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_pointers_for_null_types: true # see §13 known gaps
        emit_interface: true # generates Querier interface
        emit_db_tags: true
```

The domain package never imports anything from `generated/` or `pgx`. The adapter maps
`generated.User` → `domain.user.User` at the boundary via `mapper.go`.

### Connection pool configuration

```go
// internal/infrastructure/postgres/pool.go
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    cfg.MaxConns                 = 25
    cfg.MinConns                 = 2
    cfg.MaxConnLifetime          = 30 * time.Minute
    cfg.MaxConnIdleTime          = 5 * time.Minute
    cfg.HealthCheckPeriod        = 1 * time.Minute
    cfg.ConnConfig.ConnectTimeout = 5 * time.Second

    return pgxpool.NewWithConfig(ctx, cfg)
}
```

### Repository accepts DBTX, not pool directly

```go
// internal/adapter/persistence/postgres/user_repository.go
type UserRepository struct {
    q *db.Queries
}

// NewUserRepository accepts the sqlc-generated DBTX interface, satisfied by both
// *pgxpool.Pool and pgx.Tx. Production injects the pool; tests inject a per-test
// transaction (see §11 layer 2).
func NewUserRepository(dbtx db.DBTX) *UserRepository {
    return &UserRepository{q: db.New(dbtx)}
}
```

This single design choice makes the repository layer trivially testable against real
Postgres while remaining production-equivalent.

---

## 3. Messages — Postgres only (committed)

MongoDB is gone. Messages live in Postgres with the rest of the relational data.
Rationale: real relational shape (filter by conversation, sender, time window; unread
counts via `last_read_at`), `JSONB` for variable metadata, one transaction boundary,
one connection pool, one backup story.

### Schema

```sql
CREATE TABLE conversations (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE conversation_participants (
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_read_at    TIMESTAMPTZ,
    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       UUID        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    body            TEXT        NOT NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at       TIMESTAMPTZ
);
CREATE INDEX messages_conversation_created
    ON messages (conversation_id, created_at DESC, id DESC);

CREATE TABLE message_attachments (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id   UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    media_key    TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Domain interfaces — ISP applied

```go
// internal/domain/message/repository.go
type MessageReader interface {
    FindByConversation(
        ctx context.Context,
        conversationID shared.ID,
        cursor *Cursor,
        limit int,
    ) ([]Message, *Cursor, error)
}

type MessageWriter interface {
    Save(ctx context.Context, m Message) error
}

type ConversationRepository interface {
    EnsureParticipant(ctx context.Context, conversationID, userID shared.ID) error
    UpdateLastRead(ctx context.Context, conversationID, userID shared.ID, at time.Time) error
}
```

`ListMessages` use case depends on `MessageReader` only. `SendMessage` depends on
`MessageWriter + ConversationRepository`. The Postgres `MessageRepository` struct
implements all three. Mocks scale down to what each use case actually needs.

### Pagination SQL

```sql
-- name: ListMessagesPage :many
SELECT *
FROM messages
WHERE conversation_id = $1
  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3))
ORDER BY created_at DESC, id DESC
LIMIT $4;
```

The composite index `(conversation_id, created_at DESC, id DESC)` makes this an
index-only range scan. Offset pagination is intentionally avoided — it degrades to
O(offset + limit) and breaks when messages are inserted between requests.

---

## 4. Image Storage — Cloudflare R2 (committed)

### Why R2 for a free portfolio project

R2's free tier is permanent — it does not expire or convert to a trial. Monthly allowances:

| Resource                             | Free allowance             |
| ------------------------------------ | -------------------------- |
| Storage                              | 10 GB                      |
| Class A operations (writes/uploads)  | 1 million                  |
| Class B operations (reads/downloads) | 10 million                 |
| Egress (bandwidth out)               | **Free, always, no limit** |

> **Gotcha:** Cloudflare requires a credit card on file to enable R2, even on the free
> tier. No charges occur unless you exceed the limits above.

R2 is S3-compatible — `aws-sdk-go-v2` works against it with a custom endpoint.

### Upload constraints

| Constraint                  | Value                                      | Enforced by                              |
| --------------------------- | ------------------------------------------ | ---------------------------------------- |
| Allowed MIME types          | `image/jpeg`, `image/png`, `image/webp`    | Server before signing                    |
| Max file size               | **10 MB**                                  | Signed `Content-Length` on presigned URL |
| Presigned URL TTL           | 15 minutes                                 | `s3.WithPresignExpires`                  |
| Presign endpoint rate limit | **20 req / min, global**                   | In-process token bucket                  |
| Orphan cleanup              | R2 lifecycle rule, `pending/` prefix, 24 h | R2 bucket config                         |

### Upload flow

```
1. Client  →  POST /v1/media/upload-url   →  Go API
              (validate auth + MIME type + size)
              (sign a URL bound to Content-Length — pure CPU, ~1ms)
              return { uploadURL, mediaKey }

2. Client  →  PUT {uploadURL}  →  Cloudflare R2 directly
              (R2 rejects the upload if Content-Length or Content-Type
               differ from the signed values)

3. Client  →  PATCH /v1/messages/{id}/attachment { mediaKey }  →  Go API
              (write the key to Postgres; move object from pending/ to final prefix)
```

The Go server's compute cost per upload: one signing op + one Postgres write. Reads
hit the R2 CDN directly — the Go server never serves image bytes.

### Go implementation

```go
// internal/infrastructure/s3/client.go

func NewR2Client(cfg Config) (*s3.Client, error) {
    r2cfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            cfg.AccessKeyID, cfg.SecretAccessKey, "",
        )),
        config.WithRegion("auto"), // required by SDK, ignored by R2
        config.WithEndpointResolverWithOptions(
            aws.EndpointResolverWithOptionsFunc(func(service, region string, opts ...any) (aws.Endpoint, error) {
                return aws.Endpoint{URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)}, nil
            }),
        ),
        // ⚠️  aws-sdk-go-v2 ≥1.73.0 changed default checksum behavior — incompatible with R2.
        // Pin to 1.72.3 or disable with:
        config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
    )
    if err != nil { return nil, err }
    return s3.NewFromConfig(r2cfg), nil
}
```

```go
// internal/adapter/storage/r2/presigner.go

const MaxUploadBytes = 10 * 1024 * 1024 // 10 MB

var allowedMIMETypes = map[string]string{
    "image/jpeg": ".jpg",
    "image/png":  ".png",
    "image/webp": ".webp",
}

type Presigner struct {
    client     *s3.Client
    bucket     string
    cdnBaseURL string
}

func NewPresigner(client *s3.Client, bucket, cdnBaseURL string) *Presigner {
    return &Presigner{client: client, bucket: bucket, cdnBaseURL: cdnBaseURL}
}

func (p *Presigner) PresignUpload(
    ctx context.Context,
    userID shared.ID,
    contentType string,
    contentLength int64,
) (uploadURL, mediaKey string, err error) {
    ext, ok := allowedMIMETypes[contentType]
    if !ok {
        return "", "", media.ErrUnsupportedMediaType
    }
    if contentLength <= 0 || contentLength > MaxUploadBytes {
        return "", "", media.ErrPayloadTooLarge
    }

    // pending/ prefix is auto-swept by R2 lifecycle rule after 24h.
    // PromoteFromPending moves the object out on confirm.
    key := fmt.Sprintf("pending/%s/%s%s", userID, uuid.NewString(), ext)

    presigner := s3.NewPresignClient(p.client)
    req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
        Bucket:        aws.String(p.bucket),
        Key:           aws.String(key),
        ContentType:   aws.String(contentType),
        ContentLength: aws.Int64(contentLength),
    }, s3.WithPresignExpires(15*time.Minute))
    if err != nil {
        return "", "", fmt.Errorf("presign: %w", err)
    }
    return req.URL, key, nil
}

func (p *Presigner) PublicURL(mediaKey string) string {
    return p.cdnBaseURL + "/" + mediaKey
}
```

### Rate-limiting the presign endpoint

In-process token bucket, global (not per-user — see "Known gaps"). Cheap, stateless, no Redis.

```go
// internal/adapter/http/middleware/rate_limit.go
import "golang.org/x/time/rate"

func PresignRateLimit() func(http.Handler) http.Handler {
    lim := rate.NewLimiter(rate.Every(3*time.Second), 5) // 20/min, burst 5
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !lim.Allow() {
                http.Error(w, "rate limited", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### Orphan cleanup — R2 lifecycle rule

```json
{
  "rules": [
    {
      "id": "expire-pending-uploads",
      "enabled": true,
      "conditions": { "prefix": "pending/" },
      "deleteObjectsTransition": {
        "condition": { "type": "Age", "maxAge": 86400 }
      }
    }
  ]
}
```

Any blob under `pending/` not promoted within 24 hours is auto-deleted by R2.

### Key naming convention

```
pending/{userID}/{uuid}.webp                      ← short-lived staging
avatars/{userID}/{uuid}.webp                      ← profile pictures
messages/{conversationID}/{messageID}/{uuid}.jpg  ← message attachments
```

### CORS

```json
{
  "rules": [
    {
      "allowed": {
        "origins": [
          "http://localhost:3333"
        ],
        "methods": ["PUT"],
        "headers": ["content-type", "content-length"]
      },
      "exposeHeaders": ["ETag"],
      "maxAgeSeconds": 3000
    }
  ]
}
```

### Content-bytes trust note

The presigned URL signs `Content-Type` and `Content-Length`; R2 enforces both. The actual
_bytes_ are not validated. Mitigation: images are served from a separate origin
(`cdn.brotherband.app`) with a CSP that disallows script execution. Server-side
magic-byte sniffing is deferred (see "Known gaps").

### Domain interface

```go
// internal/domain/media/image_store.go
type ImageStore interface {
    PresignUpload(ctx context.Context, userID shared.ID, contentType string, contentLength int64) (uploadURL, mediaKey string, err error)
    PublicURL(mediaKey string) string
    PromoteFromPending(ctx context.Context, pendingKey, finalKey string) error
}
```

Use cases depend on this interface only. Tests inject a fake. Production injects the
R2 presigner.

---

## 5. Authentication — Stateless 30-day JWT in httpOnly cookies (committed)

**A single stateless JWT, 30-day expiry, HS256-signed, transported in an httpOnly +
Secure + SameSite=Lax cookie. No refresh rotation. No DB-backed token store.**

### Trade-offs of this design

This is the deliberately simplified model. The cost is real:

| Capability                        | This model                             |
| --------------------------------- | -------------------------------------- |
| Server-side logout (cookie clear) | ✅ This device only                    |
| Forced logout all devices         | ❌ Not possible (no server-side state) |
| Revoke on password change         | ❌ Existing tokens stay valid          |
| Revoke on token leak              | ❌ Token valid until natural expiry    |
| Worst-case compromise window      | **Up to 30 days**                      |

Acceptable for a portfolio project with a known, small user base. Upgrade path: add a
`token_version` column on `users`, embed it in the JWT, compare on every request (one
indexed read per call). Deferred — see "Known gaps".

### Token payload

```json
{
  "sub": "user-uuid",
  "iat": 1730000000,
  "exp": 1732592000,
  "iss": "brotherband",
  "aud": "brotherband-web"
}
```

Signed HS256 with a 32-byte secret loaded from env.

### Cookie attributes

```
Name:     bb_session
Value:    <jwt>
HttpOnly: true                 ← JS cannot read; XSS cannot steal
Secure:   true                 ← HTTPS only
SameSite: Lax                  ← blocks cross-site state-changing POSTs
Path:     /
Max-Age:  2592000              ← 30 days
Domain:   .brotherband.app
```

### CSRF — double-submit token

`SameSite=Lax` blocks most CSRF cases. For defense in depth, state-changing endpoints
require a matching CSRF token transmitted out-of-band:

1. On login, the server sets `bb_csrf` (non-httpOnly) with a random 32-byte value.
2. The SPA reads `bb_csrf` and sends it as `X-CSRF-Token` on every state-changing request.
3. Middleware rejects state-changing requests where header and cookie don't match.

An attacker on a different origin cannot read `bb_csrf` (CORS forbids it) and cannot
forge the header. Same-origin XSS defeats this — which is why CSP and input
sanitization on the SPA are non-negotiable.

```go
// internal/adapter/http/middleware/csrf.go
func CSRFCheck(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
            next.ServeHTTP(w, r); return
        }
        cookie, err := r.Cookie("bb_csrf")
        header := r.Header.Get("X-CSRF-Token")
        if err != nil || header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
            http.Error(w, "csrf token mismatch", http.StatusForbidden); return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Password hashing — argon2id

```go
// internal/adapter/auth/argon2id_hasher.go
import "golang.org/x/crypto/argon2"

const (
    argonTime    = 2
    argonMemory  = 19 * 1024  // 19 MiB — OWASP 2024 recommendation
    argonThreads = 1
    argonKeyLen  = 32
    argonSaltLen = 16
)

type Argon2idHasher struct{}

func New() *Argon2idHasher { return &Argon2idHasher{} }

// Hash returns an encoded string:
//   $argon2id$v=19$m=19456,t=2,p=1$<base64-salt>$<base64-hash>
// containing the algorithm, parameters, salt, and hash. No separate salt column
// is needed — argon2id handles salting internally and the encoded string is
// fully self-describing for verification.
func (h *Argon2idHasher) Hash(password string) (string, error) {
    salt := make([]byte, argonSaltLen)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }
    hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
    return fmt.Sprintf(
        "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, argonMemory, argonTime, argonThreads,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash),
    ), nil
}

func (h *Argon2idHasher) Verify(encoded, password string) (bool, error) {
    // Parse params, salt, and hash; recompute; subtle.ConstantTimeCompare.
    ...
}
```

The `users` table needs one column for password storage:

```sql
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL;
```

Parameter tuning is benchmarked per host — see §11 cross-cutting.

---

## 6. Database Migrations — goose (committed)

```
internal/infrastructure/postgres/migrations/
  00001_init.sql
  00002_add_conversations.sql
  00003_add_message_attachments.sql
```

```sql
-- 00001_init.sql

-- +goose Up
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT      NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    avatar_key    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
```

```go
// internal/infrastructure/postgres/migrate.go
import (
    "embed"
    "github.com/pressly/goose/v3"
    "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    db := stdlib.OpenDBFromPool(pool)
    defer db.Close()

    goose.SetBaseFS(migrationsFS)
    if err := goose.SetDialect("postgres"); err != nil {
        return err
    }
    return goose.UpContext(ctx, db, "migrations")
}
```

goose takes a Postgres advisory lock when applying migrations, so booting multiple
replicas concurrently is safe.

---

## 7. Observability — slog + request IDs + metrics (committed)

### Structured logging

`log/slog` (stdlib since Go 1.21) with the JSON handler, set as the default logger.

```go
// internal/infrastructure/observability/logger.go
func NewLogger(level slog.Level) *slog.Logger {
    h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
    l := slog.New(h)
    slog.SetDefault(l)
    return l
}
```

### Request IDs

```go
// internal/adapter/http/middleware/request_id.go
const RequestIDHeader = "X-Request-ID"
type ctxKeyRequestID struct{}

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get(RequestIDHeader)
        if id == "" {
            id = uuid.NewString()
        }
        ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, id)
        w.Header().Set(RequestIDHeader, id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func FromContext(ctx context.Context) string {
    if v, ok := ctx.Value(ctxKeyRequestID{}).(string); ok {
        return v
    }
    return ""
}
```

### Access log middleware

```go
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            next.ServeHTTP(ww, r)
            logger.LogAttrs(r.Context(), slog.LevelInfo, "http",
                slog.String("request_id", FromContext(r.Context())),
                slog.String("method", r.Method),
                slog.String("path", r.URL.Path),
                slog.Int("status", ww.Status()),
                slog.Int("bytes", ww.BytesWritten()),
                slog.Duration("duration", time.Since(start)),
            )
        })
    }
}
```

### Metrics — Prometheus

```go
// internal/infrastructure/observability/metrics.go
var (
    HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "HTTP requests by method, route, status.",
    }, []string{"method", "route", "status"})

    HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request duration.",
        Buckets: prometheus.DefBuckets,
    }, []string{"method", "route"})
)
```

Use `chi.RouteContext(r.Context()).RoutePattern()` as the `route` label so URL
parameters don't blow up cardinality.

---

## 8. Graceful Shutdown

```go
func main() {
    srv := &http.Server{
        Addr:         cfg.Addr,
        Handler:      router,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    go func() {
        if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("server error: %v", err)
        }
    }()

    <-ctx.Done()

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Printf("shutdown error: %v", err)
    }

    pgPool.Close()
    log.Println("server stopped cleanly")
}
```

---

## 9. OpenAPI — spec-first

### Backend: oapi-codegen

`api/openapi.yaml` is the source of truth. `oapi-codegen` generates Chi-compatible
server stubs, DTO types, and runtime validation middleware. The adapter layer
implements the generated interface and maps DTOs ↔ domain types.

### Frontend: @hey-api/openapi-ts

```ts
// web/openapi-ts.config.ts
import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../api/openapi.yaml",
  output: "src/lib/api",
  plugins: [
    "@hey-api/client-fetch",
    { name: "@hey-api/sdk", operationId: true },
    { name: "@tanstack/svelte-query" },
  ],
});
```

The generated client is configured at app startup to send cookies
(`credentials: 'include'`) and to read `bb_csrf` and attach it as `X-CSRF-Token` on
non-GET requests.

---

## 10. Frontend — Svelte 5 + SvelteKit + TypeScript (committed)

Svelte 5's Runes system. Reactive logic lives in `.svelte.ts` files as plain TS:

```ts
// web/src/lib/stores/user.svelte.ts
export function createUserStore() {
  let profile = $state<UserProfile | null>(null);
  let loading = $derived(profile === null);
  return {
    get profile() {
      return profile;
    },
    get loading() {
      return loading;
    },
    setProfile(p: UserProfile) {
      profile = p;
    },
  };
}
```

TanStack Query for Svelte handles server state.

**Upload flow in Svelte 5:**

```svelte
<script lang="ts">
  import { createMutation } from '@tanstack/svelte-query';
  import { requestUploadUrl, attachToMessage } from '$lib/api';

  const MAX_BYTES = 10 * 1024 * 1024;
  const ALLOWED = ['image/jpeg', 'image/png', 'image/webp'];

  const upload = createMutation({
    mutationFn: async ({ file, messageId }: { file: File; messageId: string }) => {
      if (!ALLOWED.includes(file.type)) throw new Error('Unsupported file type');
      if (file.size > MAX_BYTES) throw new Error('File exceeds 10 MB');

      const { uploadURL, mediaKey } = await requestUploadUrl({
        body: { contentType: file.type, contentLength: file.size },
      });

      const resp = await fetch(uploadURL, {
        method: 'PUT',
        headers: { 'Content-Type': file.type, 'Content-Length': String(file.size) },
        body: file,
      });
      if (!resp.ok) throw new Error('Upload to R2 failed');

      return attachToMessage({ path: { id: messageId }, body: { mediaKey } });
    },
  });
</script>
```

---

## 11. Testing strategy

Four-layer pyramid. Each layer answers a specific question; tests in the wrong layer
either retest what a lower layer already covered (slow + redundant) or skip coverage
entirely.

| Layer            | Tests               | Dependencies              | What it answers                                          | Speed target |
| ---------------- | ------------------- | ------------------------- | -------------------------------------------------------- | ------------ |
| 1 — Unit         | domain + usecase    | none (hand-rolled fakes)  | Does the business logic compute the right answer?        | <10 ms/test  |
| 2 — Repository   | adapter/persistence | real Postgres, real MinIO | Does the adapter map domain↔DB correctly under real SQL? | <100 ms/test |
| 3 — HTTP handler | adapter/http        | mocked usecase            | Wire format, status mapping, CSRF, auth all wired?       | <50 ms/test  |
| 4 — E2E          | full stack          | everything real           | Does the system satisfy the contract end-to-end?         | <2 s/test    |

CI runs all four with `go test -race ./...`. Coverage targets:

| Package                | Target                          |
| ---------------------- | ------------------------------- |
| `domain/`              | 95%+                            |
| `usecase/`             | 90%+                            |
| `adapter/http/`        | 80%+                            |
| `adapter/persistence/` | 75%+                            |
| `infrastructure/`      | not a target — exercised by E2E |

### Mocks and fakes — two conventions, intentionally different

**Domain interfaces (repositories, image store) → hand-rolled fakes** in
`internal/test/fakes/`. Simple maps with mutexes. Unit tests of use cases need _state
assertion_ (after RegisterUser, can I find the user?) — hand-rolled fakes give that
directly.

**Use-case interfaces (consumed by HTTP handlers) → generated mocks via mockery** in
`internal/test/mocks/`. Handler tests need _call assertion_ (did the handler invoke
`RegisterUser.Execute` with the right Input?) — that's exactly what `testify/mock`
provides.

Don't mix them. Hand-rolled mocks of use cases get verbose fast; generated fakes of
repos miss the point of the fake.

### Fixtures — builder pattern

```go
// internal/test/fixtures/user.go
type UserBuilder struct {
    email    string
    password string
}

func NewUser() *UserBuilder {
    return &UserBuilder{email: "default@example.com", password: "Hunter2!"}
}
func (b *UserBuilder) WithEmail(e string) *UserBuilder    { b.email = e; return b }
func (b *UserBuilder) WithPassword(p string) *UserBuilder { b.password = p; return b }

func (b *UserBuilder) Build(t *testing.T) *user.User {
    t.Helper()
    hasher := argon2id.New()
    hash, err := hasher.Hash(b.password)
    require.NoError(t, err)
    return user.MustNew(user.NewEmail(b.email), user.PasswordHash(hash))
}
```

Builders absorb changes to entity constructors — if `user.New` gains a parameter, only
the builder updates, not 200 tests.

### Layer 1 — Unit tests

Pure Go, no I/O. Hand-rolled fakes injected. Table-driven where cases share structure.

```go
func TestRegisterUser(t *testing.T) {
    cases := []struct {
        name         string
        existingUser *user.User
        input        user.RegisterUserInput
        wantErr      error
    }{
        {
            name:  "succeeds_for_new_email",
            input: user.RegisterUserInput{Email: "new@example.com", Password: "Hunter2!"},
        },
        {
            name:         "fails_when_email_taken",
            existingUser: fixtures.NewUser().WithEmail("taken@example.com").Build(t),
            input:        user.RegisterUserInput{Email: "taken@example.com", Password: "Hunter2!"},
            wantErr:      user.ErrEmailAlreadyTaken,
        },
        {
            name:    "fails_when_password_too_short",
            input:   user.RegisterUserInput{Email: "x@example.com", Password: "abc"},
            wantErr: user.ErrPasswordTooWeak,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel() // safe — each test has its own fake repo

            repo := fakes.NewUserRepo()
            if tc.existingUser != nil {
                require.NoError(t, repo.Save(context.Background(), tc.existingUser))
            }
            uc := user.NewRegisterUser(repo, argon2id.New(), clock.Fixed(t0))

            _, err := uc.Execute(context.Background(), tc.input)

            if tc.wantErr != nil {
                assert.ErrorIs(t, err, tc.wantErr)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

Naming convention: `TestThing_WhenCondition_ExpectsResult`, or descriptive case names
in table-driven. Consistent across the codebase.

### Layer 2 — Repository tests (real Postgres + real MinIO)

Containers reused across the package (`testcontainers.WithReuse()`), transaction per
test rolled back at end. No truncation, no test ordering dependencies, no inter-test
contamination.

```go
// internal/adapter/persistence/postgres/main_test.go
var pool *pgxpool.Pool

func TestMain(m *testing.M) {
    ctx := context.Background()

    pgContainer, err := containers.StartPostgres(ctx)
    if err != nil { log.Fatal(err) }
    defer pgContainer.Terminate(ctx)

    pool, err = pgxpool.New(ctx, pgContainer.DSN())
    if err != nil { log.Fatal(err) }

    if err := postgres.RunMigrations(ctx, pool); err != nil { log.Fatal(err) }

    os.Exit(m.Run())
}

// Each test gets a fresh transaction; nothing leaks between tests.
func withTx(t *testing.T) pgx.Tx {
    t.Helper()
    tx, err := pool.Begin(context.Background())
    require.NoError(t, err)
    t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
    return tx
}

func TestUserRepository_FindByEmail_ReturnsUserWhenExists(t *testing.T) {
    t.Parallel()
    tx := withTx(t)
    repo := postgres.NewUserRepository(tx) // accepts sqlc DBTX — pool or tx

    seed := fixtures.NewUser().WithEmail("found@example.com").Build(t)
    require.NoError(t, repo.Save(context.Background(), seed))

    got, err := repo.FindByEmail(context.Background(), "found@example.com")
    require.NoError(t, err)
    assert.Equal(t, seed.ID, got.ID)
}
```

**Why this works:** sqlc generates a `DBTX` interface satisfied by both `*pgxpool.Pool`
and `pgx.Tx`. Production injects the pool; tests inject a per-test transaction. The
rollback at `t.Cleanup` reverts every write the test made. Postgres provides isolation
between concurrent transactions — `t.Parallel()` is safe.

**Concrete speed** (M1 local):

- Cold container start: 3–8 s (paid once per package via `WithReuse`)
- Per-test transaction begin + rollback: ~1 ms
- Same tests with truncate-between approach: ~10–20 ms per test

### Layer 3 — HTTP handler tests

Mock the use-case interface only. The full Chi router is mounted so middleware, URL
params, CSRF, and routing execute exactly as in production.

```go
func TestRegisterUserHandler_WhenEmailTaken_Returns409(t *testing.T) {
    t.Parallel()

    uc := mocks.NewRegisterUser(t)
    uc.EXPECT().
        Execute(mock.Anything, mock.MatchedBy(func(in user.RegisterUserInput) bool {
            return in.Email == "taken@example.com"
        })).
        Return(user.RegisterUserOutput{}, user.ErrEmailAlreadyTaken)

    h := handler.NewUserHandler(uc, nil, nil)
    router := newTestRouter(h)

    body := strings.NewReader(`{"email":"taken@example.com","password":"Hunter2!"}`)
    req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
    req.Header.Set("Content-Type", "application/json")
    addCSRF(req)

    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusConflict, rec.Code)
}
```

**Tests at this layer must include:**

- Validation failures (empty body, missing field, malformed JSON) → 400
- Auth middleware (missing/invalid cookie) → 401
- CSRF rejection (missing/mismatched token) → 403
- Domain error → HTTP status, exhaustively table-driven over the full error map
- Response shape (golden JSON for stable contracts — deferred until OpenAPI stabilizes)

**The error map test is the highest-value single test at this layer:**

```go
func TestErrorMapping(t *testing.T) {
    cases := []struct {
        domainErr error
        wantCode  int
    }{
        {user.ErrEmailAlreadyTaken,       http.StatusConflict},
        {user.ErrInvalidCredentials,      http.StatusUnauthorized},
        {user.ErrPasswordTooWeak,         http.StatusUnprocessableEntity},
        {shared.ErrNotFound,              http.StatusNotFound},
        {media.ErrPayloadTooLarge,        http.StatusRequestEntityTooLarge},
        {media.ErrUnsupportedMediaType,   http.StatusUnsupportedMediaType},
        {message.ErrNotParticipant,       http.StatusForbidden},
    }
    for _, tc := range cases {
        t.Run(tc.domainErr.Error(), func(t *testing.T) {
            rec := httptest.NewRecorder()
            errormap.Write(rec, tc.domainErr)
            assert.Equal(t, tc.wantCode, rec.Code)
        })
    }
}
```

When a new domain error is added and someone forgets to map it, this test fails with
a clear "no mapping found" signal. The error map is the contract between domain and
HTTP — test it explicitly.

### Layer 4 — End-to-end tests

Full stack via `httptest.NewServer`. All containers real. Tests the actual sequence as
a browser would experience it.

```go
func TestMessageAttachmentFlow(t *testing.T) {
    srv := startTestServer(t)
    defer srv.Close()
    c := httpexpect.Default(t, srv.URL)

    sessionCookie, csrfToken := loginAndGetSession(c, "a@b.com", "Hunter2!")

    upload := c.POST("/v1/media/upload-url").
        WithCookie("bb_session", sessionCookie).
        WithHeader("X-CSRF-Token", csrfToken).
        WithJSON(map[string]any{
            "contentType":   "image/webp",
            "contentLength": len(testImageBytes),
        }).
        Expect().Status(http.StatusOK).JSON().Object()

    req, _ := http.NewRequest(http.MethodPut, upload.Value("uploadURL").String().Raw(),
        bytes.NewReader(testImageBytes))
    req.Header.Set("Content-Type", "image/webp")
    req.ContentLength = int64(len(testImageBytes))
    resp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp.StatusCode)

    c.PATCH("/v1/messages/{id}/attachment", messageID).
        WithCookie("bb_session", sessionCookie).
        WithHeader("X-CSRF-Token", csrfToken).
        WithJSON(map[string]string{"mediaKey": upload.Value("mediaKey").String().Raw()}).
        Expect().Status(http.StatusOK)
}
```

**E2E covers explicit rejection paths the lower layers can only simulate:**

- Upload PUT with mismatched `Content-Length` → R2 returns 400 (signed-length enforcement)
- Upload PUT after URL expiry → R2 returns 403
- Presign request with `contentLength > 10 MB` → server returns 413
- Login → presign → upload → attach sequence cross-cookie + cross-CSRF

E2E tests run sequentially (not `t.Parallel`) unless explicitly isolated by per-test
user/conversation — the overhead of orchestrating isolation usually isn't worth it for
this layer.

### Cross-cutting

**Race detector.** All CI runs `go test -race ./...`. The in-process rate limiter, the
request ID middleware, and the metrics collectors share state across goroutines; race
detection is non-negotiable.

**Parallelism rules:**

- Unit tests — `t.Parallel()` safe; each test has its own fakes.
- Handler tests — `t.Parallel()` safe; each test has its own mock + recorder.
- Repository tests — `t.Parallel()` safe _if_ every test uses `withTx`. Postgres handles
  the concurrency; transactions don't see each other's writes.
- E2E — sequential by default.

**Clock injection.** `usecase/port.Clock` is injected everywhere time matters. JWT
expiry tests, message ordering tests, and cursor pagination tests all depend on
`clock.Fixed(t0)` in tests vs `clock.System()` in prod. No `time.Now()` calls in
business logic.

**Argon2id benchmark.** Hash time is hardware-dependent. Run the benchmark on the
target host and tune `argonMemory` / `argonTime` until hash time falls in the
100–300 ms band (OWASP target).

```go
// internal/adapter/auth/argon2id_hasher_test.go
func BenchmarkArgon2idHash(b *testing.B) {
    h := New()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = h.Hash("Hunter2!")
    }
}
```

```
go test -bench=. -benchtime=3s ./internal/adapter/auth/...
```

Aim for ~200 ms/op on production hardware. Lower → raise memory cost; higher → lower
memory cost.

**Test data lifetime.** Layer 2 tests rely entirely on transaction rollback for
isolation. Don't introduce truncation in `TestMain` — that defeats the parallelism
benefit and is slower. Don't seed shared data in `TestMain` either; seed per test
inside the transaction.

---

## Decision log

| Concern                   | Decision                                                             | Rationale                                                              |
| ------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Layer map                 | Strict Clean with aggregate-scoped domain packages                   | SRP at package level; ISP via split interfaces per aggregate           |
| Use-case shape            | One struct per operation with own Input/Output DTOs                  | SRP; mock surface stays minimal                                        |
| Cross-cutting ports       | Live in `usecase/port/`                                              | Inner layer declares the interface, outer layer implements             |
| Adapter vs Infrastructure | Adapter implements an inner interface; Infrastructure is pure wiring | Eliminates the auth/JWT placement ambiguity                            |
| HTTP router               | Chi                                                                  | Pure `net/http`; explicit middleware; smaller surface than Echo        |
| Postgres access           | sqlc 1.31+ + pgx/v5 with `emit_interface`                            | Compile-time SQL safety; DBTX interface enables tx-per-test            |
| pgx pool                  | `MaxConns=25`, lifetime/idle/healthcheck tuned                       | Explicit limits avoid silent saturation                                |
| Single store              | Postgres for everything (incl. messages)                             | Operational simplification; relational queries on messages first-class |
| Message metadata          | `JSONB` column                                                       | Variable shape, no schema cost                                         |
| Message pagination        | Cursor (`created_at`, `id`), DESC                                    | O(limit) regardless of position; stable across writes                  |
| Image storage             | Cloudflare R2 + aws-sdk-go-v2                                        | Free tier: 10 GB + 1M writes + 10M reads/month, zero egress            |
| Image upload pattern      | Presigned PUT URL, size + type bound                                 | Client uploads direct to R2; R2 rejects mismatched headers             |
| Upload size cap           | 10 MB, jpg/png/webp only                                             | Hard limit signed into URL                                             |
| Presign rate limit        | Global token bucket, 20 req/min                                      | In-process, no state, no Redis                                         |
| Orphan blobs              | R2 lifecycle rule, `pending/` 24 h                                   | Auto-cleanup of incomplete uploads                                     |
| Auth                      | Stateless 30-day JWT in httpOnly cookie                              | Simplification; no DB lookup on each request                           |
| JWT algorithm             | HS256                                                                | Symmetric, monolith-only verification                                  |
| CSRF                      | SameSite=Lax + double-submit token                                   | Belt and suspenders                                                    |
| Password hashing          | argon2id (OWASP-2024 params, host-tuned)                             | Salt + params embedded in encoded string; no separate column           |
| Schema migrations         | goose, embedded, advisory-locked at boot                             | Transactional by default; minimal moving parts                         |
| Observability             | slog (JSON) + request ID + Prometheus                                | Structured logs, correlation, basic metrics surface                    |
| Backend OpenAPI           | oapi-codegen, spec-first                                             | Generated Chi-compatible server stubs and DTOs                         |
| Frontend client           | @hey-api/openapi-ts + TanStack Svelte Query                          | Type-safe generated client; native Svelte 5 integration                |
| Frontend framework        | Svelte 5 + SvelteKit + TypeScript                                    | Stable Runes reactivity; first-class TS                                |
| Test layer-1 doubles      | Hand-rolled fakes for domain interfaces                              | State assertion needed, not call assertion                             |
| Test layer-3 doubles      | mockery-generated mocks for use cases                                | Call assertion is the value at this layer                              |
| Test layer-2 isolation    | Transaction-per-test via sqlc DBTX                                   | ~1 ms vs ~10–20 ms truncate; parallelism-safe                          |
| Test fixtures             | Builder pattern in `internal/test/fixtures/`                         | Absorbs constructor changes                                            |
| CI test flags             | `-race` always on                                                    | Catches data races in middleware + rate limiter                        |
| Argon2id tuning           | Benchmark on target host, aim 100–300 ms/op                          | Hardware-dependent; OWASP-recommended band                             |

---

## Known gaps / deferred

Explicit accepted trade-offs. Not silent absences — known and chosen.

1. **No JWT revocation.** A leaked `bb_session` cookie is valid until 30-day expiry.
   Password change doesn't invalidate existing tokens. Upgrade path: `token_version`
   column on `users`, embedded in JWT, compared on every request (one indexed read).

2. **No per-user rate limiting.** Global rate limit on the presign endpoint blunts
   the worst case. Upgrade path: in-process LRU keyed by `user_id`, or Postgres
   `rate_limit_buckets` with a sliding-window counter.

3. **No magic-byte validation on uploaded images.** MIME type is asserted by the client
   and enforced by R2 on header match only. Mitigated by separate CDN origin + CSP.
   Upgrade path: server-side `http.DetectContentType` on the first 512 bytes after
   upload, delete on mismatch.

4. **No distributed tracing.** Single-service deployment doesn't need it. Upgrade
   path: OpenTelemetry SDK + OTLP exporter + tracing middleware on Chi.

5. **No background job runner.** Anything async (reconciliation sweeps, scheduled
   cleanups) is deferred. Upgrade path: `riverqueue/river` — Postgres-backed.

6. **`emit_pointers_for_null_types: true`** — pointer-typed nullable columns lose
   the distinction between absent and `null` in JSON. Acceptable here; switch to
   `pgtype` if API semantics require it.

7. **Snapshot/golden testing for HTTP responses.** Useful for stable public contracts;
   overkill while OpenAPI is still iterating. Add when responses become a public
   contract.

8. **Mutation testing.** `gremlins` exists but adds complexity for marginal extra
   confidence at this scope.
