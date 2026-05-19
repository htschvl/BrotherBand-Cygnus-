# BrotherBand — Technical Documentation

| Field        | Value                              |
| ------------ | ---------------------------------- |
| Tech lead    | Clarice                            |
| Repos        | Monorepo (`backend`, `web`, `api`) |
| Environments | Local dev / staging / production   |
| Last updated | 2026-05-11                         |
| Status       | Active architecture design         |

---

## 1. System Overview

BrotherBand is a minimalist social networking platform focused on trusted small-circle relationships instead of mass social interaction. Technically, the system is a spec-first monolithic architecture built around Clean Architecture principles using Go, Postgres, Cloudflare R2, and a SvelteKit frontend. The backend exposes a REST API with JWT cookie authentication, direct-to-storage media uploads via presigned URLs, structured observability, and strong testing isolation through transaction-scoped integration testing.

### Key design principles

- Strict Clean Architecture boundaries.
- Spec-first API development via OpenAPI.
- Minimal operational complexity.
- Single primary datastore (Postgres).
- Stateless authentication.
- Direct-to-storage media uploads.
- High testability and dependency inversion.
- Intentional simplicity over premature scale optimization.

---

## 2. Architecture

### 2.1 Components

| Component         | Responsibility                                                 | Tech                            | Repo                                    |
| ----------------- | -------------------------------------------------------------- | ------------------------------- | --------------------------------------- |
| API Backend       | Business logic, authentication, messaging, media orchestration | Go, Chi, sqlc, pgx/v5           | `internal/`, `cmd/api`                  |
| Frontend          | User interface and SPA behavior                                | Svelte 5, SvelteKit, TypeScript | `web/`                                  |
| Database          | Persistent relational storage                                  | PostgreSQL                      | Infrastructure                          |
| Media Storage     | Image upload and delivery                                      | Cloudflare R2                   | Infrastructure                          |
| API Specification | Source of truth for contracts                                  | OpenAPI                         | `api/openapi.yaml`                      |
| Observability     | Metrics and structured logging                                 | Prometheus, slog                | `internal/infrastructure/observability` |

### 2.2 Data Flow

#### Authentication flow

1. User logs in through REST API.
2. Backend validates credentials with Argon2id.
3. JWT is issued and stored in a secure httpOnly cookie.
4. CSRF token is issued separately for state-changing requests.

#### Media upload flow

1. Client requests presigned upload URL.
2. Backend validates MIME type and size.
3. Client uploads directly to Cloudflare R2.
4. Backend confirms and promotes uploaded media.

#### Messaging flow

1. User sends message through REST API.
2. Use case validates participation.
3. Repository persists message in Postgres.
4. Cursor pagination retrieves conversations efficiently.

### 2.3 Trust & Boundaries

| Boundary            | Trust Level                                                 |
| ------------------- | ----------------------------------------------------------- |
| Browser client      | Untrusted                                                   |
| HTTP API            | Trusted application boundary                                |
| Postgres            | Trusted persistent store                                    |
| Cloudflare R2       | Trusted object storage                                      |
| JWT cookie          | Trusted until expiration                                    |
| Uploaded file bytes | Semi-trusted; MIME enforced but content not fully validated |

---

## 3. Tech Stack

| Layer           | Technology                | Version        | Notes                      |
| --------------- | ------------------------- | -------------- | -------------------------- |
| Frontend        | Svelte 5 + SvelteKit      | Latest stable  | SPA + reactive state       |
| Backend         | Go + Chi                  | Go 1.21+       | REST API                   |
| API Contracts   | OpenAPI                   | Spec-first     | Source of truth            |
| Database Access | sqlc + pgx/v5             | sqlc 1.31+     | No ORM                     |
| Database        | PostgreSQL                | Current stable | Single datastore           |
| Storage         | Cloudflare R2             | S3-compatible  | Media uploads              |
| Auth            | JWT + Argon2id            | HS256          | Stateless sessions         |
| Observability   | slog + Prometheus         | Go stdlib      | Structured metrics/logging |
| Testing         | Testcontainers + httptest | Current stable | Four-layer testing         |

---

## 4. Data Model

### Core entities

- Users
- Conversations
- Conversation Participants
- Messages
- Message Attachments
- Media Assets

### Relationships

- Users participate in conversations.
- Conversations contain messages.
- Messages may contain media attachments.
- Media references are stored relationally while bytes live in R2.

### Indexing Strategy

Primary optimization:

```sql
(conversation_id, created_at DESC, id DESC)
```

Used for efficient cursor-based pagination.

---

## 5. Interfaces

### 5.1 External APIs

| API Type              | Purpose                        |
| --------------------- | ------------------------------ |
| REST                  | Core application functionality |
| Presigned Upload URLs | Direct media upload            |
| Metrics Endpoint      | Prometheus scraping            |

Authentication:

- JWT in httpOnly cookies.
- CSRF protection using double-submit token pattern.

### 5.2 Internal Services

| Service           | Responsibility               |
| ----------------- | ---------------------------- |
| User Use Cases    | Registration, login, profile |
| Message Use Cases | Sending and listing messages |
| Media Use Cases   | Upload orchestration         |

### 5.3 Service Contracts

- Use-case-per-struct pattern.
- DTO isolation between layers.
- Dependency inversion via ports/interfaces.

---

## 6. Infrastructure & Deployment

### Hosting

- Backend hosted as stateless Go service.
- Postgres as primary database.
- Cloudflare R2 for object storage.
- CDN-backed media delivery.

### CI/CD

- Spec-first code generation.
- Automated testing across all layers.
- Migration execution at startup using goose.

### Secrets Management

Environment variables:

- JWT secret
- R2 credentials
- Database DSN

### Deployment Flow

- Development
- Staging
- Production

---

## 7. Security

### Threat Model

| Threat               | Mitigation                              |
| -------------------- | --------------------------------------- |
| XSS token theft      | httpOnly cookies                        |
| CSRF                 | SameSite=Lax + CSRF token               |
| Password compromise  | Argon2id hashing                        |
| Oversized uploads    | Signed content-length                   |
| Unauthorized uploads | Presigned URL expiration                |
| Toxic file uploads   | MIME restrictions + isolated CDN origin |

### Authentication & Authorization

- Stateless JWT auth.
- Conversation participation validation.
- Role-less ownership model.

### Known Limitations

- No JWT revocation.
- No magic-byte media validation.
- No distributed tracing.
- No per-user upload rate limiting.

---

## 8. Observability

### Logging

- Structured JSON logging via `slog`.
- Request IDs propagated through middleware.

### Metrics

- Prometheus counters and histograms.
- HTTP duration tracking.
- Request status monitoring.

### Alerts & Monitoring

Current architecture supports:

- Error rate monitoring.
- Latency monitoring.
- Request throughput tracking.

---

## 9. Performance & Scalability

### Current Scalability Strategy

- Stateless backend scaling.
- Direct-to-storage uploads.
- Cursor pagination.
- Single relational datastore.

### Bottlenecks

- Single Postgres instance.
- Stateless JWT revocation limitations.
- In-process global rate limiter.

### Scaling Levers

- Read replicas.
- CDN edge delivery.
- Background jobs.
- Per-user distributed rate limiting.

---

## 10. Dependencies

| Dependency     | Purpose             | Criticality | Fallback                  |
| -------------- | ------------------- | ----------- | ------------------------- |
| PostgreSQL     | Primary datastore   | Critical    | Backups/restores          |
| Cloudflare R2  | Media storage       | High        | S3-compatible replacement |
| OpenAPI        | API contracts       | High        | Manual DTO maintenance    |
| Prometheus     | Metrics collection  | Medium      | Log-based observability   |
| Testcontainers | Integration testing | Medium      | Local Docker services     |

---

## 11. Operational Runbooks

### Common Incidents

- Database connection exhaustion.
- Failed migrations.
- R2 upload failures.
- JWT secret mismatch.
- Expired presigned uploads.

### Handling Strategy

- Structured logs with request IDs.
- Graceful shutdown support.
- Transaction rollback isolation.
- Migration locking through goose.

---

## 12. Technical Debt & Known Issues

| Issue                      | Severity | Planned Resolution  |
| -------------------------- | -------- | ------------------- |
| No JWT revocation          | Medium   | Token versioning    |
| No distributed tracing     | Low      | OpenTelemetry       |
| No background job runner   | Medium   | River queue         |
| No content-byte validation | Medium   | Magic-byte sniffing |
| Global rate limiting only  | Medium   | Per-user limits     |
| Nullable pointer ambiguity | Low      | pgtype migration    |

Primary accepted tradeoff:
The architecture intentionally prioritizes simplicity, maintainability, and low operational overhead over enterprise-scale complexity.
