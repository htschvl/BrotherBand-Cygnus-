# BrotherBand · Cygnus — Frontend Implementation Guide (Svelte 5 + TypeScript)

This is the authoritative guide for building the BrotherBand web client against the
Cygnus backend. It is written for **Svelte 5 (runes)** + **SvelteKit** +
**TypeScript**, with a **type-safe API client generated from
[`api/openapi.yaml`](../api/openapi.yaml)** — which the backend audit confirms is
accurate to the running server (every path, schema, status code, and the
auth/CSRF model match the implementation field-for-field).

Read this top to bottom once; every section builds on the auth/CSRF model in §3.

---

## Table of contents

1. [What you are building against](#1-what-you-are-building-against)
2. [Project bootstrap](#2-project-bootstrap)
3. [The auth & CSRF model (read this first)](#3-the-auth--csrf-model-read-this-first)
4. [Generating the type-safe client](#4-generating-the-type-safe-client)
5. [Configuring the client: cookies + CSRF interceptor](#5-configuring-the-client-cookies--csrf-interceptor)
6. [Server state with TanStack Query](#6-server-state-with-tanstack-query)
7. [The auth store (Svelte 5 runes)](#7-the-auth-store-svelte-5-runes)
8. [Error handling: the uniform envelope](#8-error-handling-the-uniform-envelope)
9. [Feature recipes](#9-feature-recipes)
   - 9.1 [Register / Login / Logout](#91-register--login--logout)
   - 9.2 [The session guard & route protection](#92-the-session-guard--route-protection)
   - 9.3 [Brotherband: send / accept (the secret reveal) / deny / cut](#93-brotherband-send--accept-the-secret-reveal--deny--cut)
   - 9.4 [Messaging with cursor pagination](#94-messaging-with-cursor-pagination)
   - 9.5 [Media upload (3-step direct-to-R2 flow)](#95-media-upload-3-step-direct-to-r2-flow)
10. [Suggested routing structure](#10-suggested-routing-structure)
11. [Environment & CORS (dev and prod)](#11-environment--cors-dev-and-prod)
12. [Endpoint → SDK function reference](#12-endpoint--sdk-function-reference)
13. [Gotchas specific to this API](#13-gotchas-specific-to-this-api)

---

## 1. What you are building against

| Fact             | Value                                                                                                                         |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| API base (dev)   | `http://localhost:3000`                                                                                                       |
| API contract     | `api/openapi.yaml` (OpenAPI 3.1) — the single source of truth                                                                 |
| Auth transport   | **cookies**, not `Authorization` headers                                                                                      |
| Session cookie   | `bb_session` — **httpOnly** (JS cannot read it), JWT, 30-day                                                                  |
| CSRF cookie      | `bb_csrf` — readable by JS, 32-byte random                                                                                    |
| CSRF requirement | echo `bb_csrf` as the `X-CSRF-Token` header on **state-changing** requests only (POST/PATCH/DELETE) — **except** `/v1/auth/*` |
| Error shape      | uniform envelope: `{ code, message, requestId, details? }`                                                                    |
| IDs              | UUID strings                                                                                                                  |
| Timestamps       | RFC 3339 strings                                                                                                              |
| Pagination       | opaque cursor string (never parse it)                                                                                         |

The recommended stack (matches the backend architecture doc):

- **SvelteKit** + **Svelte 5** (runes: `$state`, `$derived`, `$effect`)
- **TypeScript** (strict)
- **`@hey-api/openapi-ts`** — generates the typed client + SDK from the spec
- **`@hey-api/client-fetch`** — the fetch runtime for the generated client
- **`@tanstack/svelte-query`** — server-state cache/mutations

---

## 2. Project bootstrap

```bash
# from the monorepo root, alongside the Go backend
npm create svelte@latest web      # choose: SvelteKit skeleton, TypeScript
cd web
npm install
npm install -D @hey-api/openapi-ts
npm install @hey-api/client-fetch @tanstack/svelte-query
```

`web/` is a separate build with its own CI — it never imports Go code. Its only
contract with the backend is `api/openapi.yaml`.

Recommended `web/` layout:

```
web/
├── openapi-ts.config.ts          # codegen config (points at ../api/openapi.yaml)
├── src/
│   ├── lib/
│   │   ├── api/                  # GENERATED — do not edit by hand
│   │   ├── client.ts             # configures the generated client (cookies + CSRF)
│   │   ├── query.ts              # QueryClient factory
│   │   ├── errors.ts             # ApiError type + helpers over the envelope
│   │   └── stores/
│   │       └── session.svelte.ts # auth state (runes)
│   ├── routes/                   # SvelteKit routes
│   └── app.d.ts
└── package.json
```

---

## 3. The auth & CSRF model (read this first)

Everything in the client depends on understanding this exactly.

1. **`POST /v1/auth/register`** or **`POST /v1/auth/login`** returns the user profile
   JSON and sets two cookies via `Set-Cookie`:
   - `bb_session` — **httpOnly**. Your JS will **never** read this. The browser
     attaches it automatically to every request **iff** the request is made with
     `credentials: 'include'`.
   - `bb_csrf` — **readable**. Your JS reads this from `document.cookie` and echoes
     it as the `X-CSRF-Token` header on every state-changing request.

2. **State-changing requests** (`POST`, `PATCH`, `DELETE`) to authenticated endpoints
   must send `X-CSRF-Token: <value of bb_csrf cookie>`. The server does a
   constant-time compare; a mismatch is `403 csrf.mismatch`.

3. **`GET` requests need no CSRF header** (only the session cookie). Sending one is
   harmless but unnecessary.

4. **`/v1/auth/register`, `/v1/auth/login`, `/v1/auth/logout` are CSRF-exempt** — they
   are the endpoints that _issue_ `bb_csrf`, so they cannot require it. Do **not**
   send `X-CSRF-Token` to them (it is simply ignored).

5. **There is no "get me a token" endpoint and no localStorage token.** Auth state is
   entirely cookie-driven. To know "am I logged in?", call `GET /v1/me`: `200` → yes,
   `401` → no. (You cannot read `bb_session` to check — it's httpOnly by design, so
   XSS cannot exfiltrate it.)

6. **Logout** is `POST /v1/auth/logout` — it clears both cookies server-side. There is
   no client-side token to discard.

> Security consequence you must respect: because the session cookie is httpOnly, the
> only XSS-exploitable surface is `bb_csrf`. Keep the SvelteKit CSP strict and never
> `{@html}` untrusted content.

---

## 4. Generating the type-safe client

`web/openapi-ts.config.ts`:

```ts
import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../api/openapi.yaml",
  output: { path: "src/lib/api", format: "prettier" },
  plugins: [
    "@hey-api/client-fetch",
    { name: "@hey-api/sdk", operationId: true }, // function names from operationId
    "@hey-api/typescript", // request/response types
  ],
});
```

Add to `web/package.json`:

```json
{
  "scripts": {
    "gen": "openapi-ts",
    "dev": "npm run gen && vite dev",
    "build": "npm run gen && vite build"
  }
}
```

`npm run gen` produces, in `src/lib/api/`:

- **`types.gen.ts`** — every schema as a TS type (`UserProfile`, `Message`,
  `BrotherSummary`, `Error`, …).
- **`sdk.gen.ts`** — one async function per `operationId`
  (`registerUser`, `loginUser`, `getMyProfile`, `sendMessageToBrother`,
  `requestUploadUrl`, …). Each returns `{ data, error, response }`.
- **`client.gen.ts`** — the configurable fetch client instance.

Re-run `npm run gen` whenever `api/openapi.yaml` changes. The build script does it
automatically, so the types can never drift from the backend contract.

---

## 5. Configuring the client: cookies + CSRF interceptor

This is the single most important file. Two responsibilities: **(a)** send cookies on
every request; **(b)** attach `X-CSRF-Token` on state-changing requests.

`web/src/lib/client.ts`:

```ts
import { client } from "$lib/api/client.gen";

/** Read a non-httpOnly cookie by name (bb_csrf is readable; bb_session is not). */
function readCookie(name: string): string | null {
  if (typeof document === "undefined") return null; // SSR guard
  const match = document.cookie.match(
    new RegExp(
      "(?:^|; )" + name.replace(/[.$?*|{}()[\]\\/+^]/g, "\\$&") + "=([^;]*)",
    ),
  );
  return match ? decodeURIComponent(match[1]) : null;
}

const STATE_CHANGING = new Set(["POST", "PUT", "PATCH", "DELETE"]);

export function configureApiClient(baseUrl: string) {
  client.setConfig({
    baseUrl,
    // (a) send & receive the bb_session / bb_csrf cookies cross-origin.
    credentials: "include",
  });

  // (b) double-submit CSRF: echo the bb_csrf cookie as X-CSRF-Token on
  //     state-changing requests. Auth endpoints are CSRF-exempt, and the
  //     server ignores the header there, so it is safe to always attach it
  //     when present.
  client.interceptors.request.use((request) => {
    if (STATE_CHANGING.has(request.method.toUpperCase())) {
      const csrf = readCookie("bb_csrf");
      if (csrf) request.headers.set("X-CSRF-Token", csrf);
    }
    return request;
  });
}
```

Call it once at app startup — `web/src/routes/+layout.ts`:

```ts
import { configureApiClient } from "$lib/client";
import { PUBLIC_API_BASE_URL } from "$env/static/public";

export const ssr = false; // this client is cookie/CSR-based; render on the client

configureApiClient(PUBLIC_API_BASE_URL); // e.g. http://localhost:3000
```

> **Why `ssr = false`:** auth is cookie-based and `bb_csrf` is read from
> `document.cookie`. Doing the authed calls in the browser keeps the model simple and
> correct. If you later want SSR, you must forward the incoming `cookie` header from
> the SvelteKit server hooks into the fetch — out of scope for the first iteration.

---

## 6. Server state with TanStack Query

`web/src/lib/query.ts`:

```ts
import { QueryClient } from "@tanstack/svelte-query";

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (count, err) => {
          // Never retry auth/permission/validation failures — they are
          // deterministic given the same input.
          const status = (err as { status?: number })?.status;
          if (status && [400, 401, 403, 404, 409, 415, 422].includes(status))
            return false;
          return count < 2;
        },
      },
    },
  });
}
```

`web/src/routes/+layout.svelte`:

```svelte
<script lang="ts">
  import { QueryClientProvider } from '@tanstack/svelte-query';
  import { makeQueryClient } from '$lib/query';
  let { children } = $props();
  const qc = makeQueryClient();
</script>

<QueryClientProvider client={qc}>
  {@render children()}
</QueryClientProvider>
```

---

## 7. The auth store (Svelte 5 runes)

Reactive state lives in a `.svelte.ts` module as plain runes — no legacy stores.

`web/src/lib/stores/session.svelte.ts`:

```ts
import type { UserProfile } from "$lib/api/types.gen";
import { getMyProfile, logoutUser } from "$lib/api/sdk.gen";

function createSession() {
  let profile = $state<UserProfile | null>(null);
  let loaded = $state(false); // false until the first /v1/me probe resolves

  return {
    get profile() {
      return profile;
    },
    get isAuthenticated() {
      return profile !== null;
    },
    get loaded() {
      return loaded;
    },

    /** Probe the session on app start / after auth mutations. */
    async refresh() {
      const { data, error } = await getMyProfile();
      profile = error ? null : (data ?? null);
      loaded = true;
    },

    /** Called by login/register flows with the returned profile. */
    set(p: UserProfile) {
      profile = p;
      loaded = true;
    },

    async logout() {
      await logoutUser(); // server clears bb_session + bb_csrf
      profile = null;
    },
  };
}

export const session = createSession();
```

Probe once at startup (extend `+layout.ts`):

```ts
import { session } from "$lib/stores/session.svelte";
// ... after configureApiClient(...)
await session.refresh();
```

---

## 8. Error handling: the uniform envelope

Every non-2xx response is the **same shape** (verified against the server's
`respond.ErrorBody`):

```ts
// matches components.schemas.Error in the spec
export interface ApiErrorBody {
  code: string; // STABLE — switch on this, never on `message`
  message: string; // human-readable, safe to show
  requestId?: string; // put this in bug reports / support tickets
  details?: {
    field?: string; // present on 422 validation failures
    reason?: string;
    fields?: { field: string; reason: string }[]; // batch validation
    [k: string]: unknown;
  };
}
```

`web/src/lib/errors.ts`:

```ts
import type { ApiErrorBody } from "./errors.types";

/** hey-api returns { data, error }. `error` is the parsed ApiErrorBody. */
export function isApiError(e: unknown): e is ApiErrorBody {
  return !!e && typeof e === "object" && "code" in e && "message" in e;
}

/** Map a code to a user-facing message. Switch on CODE, not message text. */
export function humanize(err: ApiErrorBody): string {
  switch (err.code) {
    case "user.username_taken":
      return "That username is already taken.";
    case "user.invalid_credentials":
      return "Wrong username or password.";
    case "user.password_too_weak":
      return "Password must be 8–128 characters.";
    case "brotherband.already_brothers":
      return "You are already brothers.";
    case "brotherband.request_exists":
      return "A request is already pending.";
    case "brotherband.not_a_brother":
      return "You are not brothers with this user.";
    case "brotherband.not_recipient":
      return "Only the recipient can act on this request.";
    case "message.invalid_body":
      return "Message cannot be empty.";
    case "media.payload_too_large":
      return "Image must be at most 10 MB.";
    case "media.unsupported_type":
      return "Only JPEG, PNG or WebP are allowed.";
    case "csrf.mismatch":
      return "Your session expired — please reload.";
    case "rate_limited":
      return "Too many requests — slow down a moment.";
    default:
      // field-level validation detail, when present
      if (err.details?.reason)
        return `Invalid ${err.details.field}: ${err.details.reason}`;
      return err.message;
  }
}
```

The full stable `code` registry is in the API description inside `api/openapi.yaml`.
A `401` anywhere means "session gone" → clear the session store and route to login
(see §9.2). Always surface `requestId` in unexpected-error UI ("Reference: …") — it
correlates one-to-one with the server log line.

---

## 9. Feature recipes

All snippets assume the generated SDK functions and the configured client.

### 9.1 Register / Login / Logout

```svelte
<script lang="ts">
  import { registerUser, loginUser } from '$lib/api/sdk.gen';
  import { session } from '$lib/stores/session.svelte';
  import { isApiError, humanize } from '$lib/errors';
  import { goto } from '$app/navigation';

  let username = $state('');
  let password = $state('');
  let errorMsg = $state<string | null>(null);
  let busy = $state(false);

  async function submitLogin() {
    busy = true; errorMsg = null;
    const { data, error } = await loginUser({ body: { username, password } });
    busy = false;
    if (error) { errorMsg = isApiError(error) ? humanize(error) : 'Login failed.'; return; }
    session.set(data!);          // cookies are already set by Set-Cookie
    await goto('/app');
  }
</script>
```

Registration is identical but calls `registerUser({ body: { username, password,
birthdate, secret, status, favorites } })`. **Validation rules the form must enforce
(mirrors the server; a violation returns `422` with `details.field`):**

| Field       | Rule                                             |
| ----------- | ------------------------------------------------ |
| `username`  | 3–32 chars; letters/digits/`_`/`-`; unique       |
| `password`  | 8–128 chars                                      |
| `birthdate` | `YYYY-MM-DD`                                     |
| `secret`    | 1–280 chars                                      |
| `status`    | 1–280 chars                                      |
| `favorites` | **exactly 5** non-empty strings, each ≤ 80 chars |

Logout:

```ts
await session.logout();
await goto("/login");
```

### 9.2 The session guard & route protection

Put protected pages under a route group whose `+layout.ts` enforces auth:

`web/src/routes/(app)/+layout.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import { session } from "$lib/stores/session.svelte";

export const ssr = false;

export async function load() {
  if (!session.loaded) await session.refresh();
  if (!session.isAuthenticated) throw redirect(302, "/login");
}
```

Add a global response interceptor so an expired session anywhere bounces to login:

```ts
// in client.ts, after the request interceptor
client.interceptors.response.use((response) => {
  if (response.status === 401 && typeof window !== "undefined") {
    // session gone — drop local state; route guard handles the redirect
    import("$lib/stores/session.svelte").then((m) =>
      m.session.set(null as never),
    );
    if (location.pathname !== "/login") location.assign("/login");
  }
  return response;
});
```

### 9.3 Brotherband: send / accept (the secret reveal) / deny / cut

The product's core ritual: when you **accept** a request, the server returns the
requester's **secret exactly once**. There is no endpoint that returns it again — you
must capture and persist it client-side at accept time.

```svelte
<script lang="ts">
  import {
    sendBrotherbandRequest, acceptBrotherbandRequest,
    denyBrotherbandRequest, cutBrotherband,
  } from '$lib/api/sdk.gen';
  import { createMutation, useQueryClient } from '@tanstack/svelte-query';

  let { recipientId, requestId }: { recipientId: string; requestId: string } = $props();
  const qc = useQueryClient();

  const accept = createMutation({
    mutationFn: () => acceptBrotherbandRequest({ path: { requestId } }),
    onSuccess: ({ data }) => {
      // ⚠️ data.requesterSecret is shown ONCE. Persist/display it now —
      // it is never returned by any future call.
      revealSecretModal(data!.brother.username, data!.requesterSecret);
      qc.invalidateQueries({ queryKey: ['brothers'] });
      qc.invalidateQueries({ queryKey: ['brotherband-requests'] });
    },
  });
</script>

<button onclick={() => sendBrotherbandRequest({ path: { recipientId } })}>Add</button>
<button onclick={() => $accept.mutate()}>Accept</button>
<button onclick={() => denyBrotherbandRequest({ path: { requestId } })}>Deny</button>
<button onclick={() => cutBrotherband({ path: { brotherId: recipientId } })}>Cut</button>
```

Pending requests list:

```ts
import { createQuery } from "@tanstack/svelte-query";
import { listBrotherbandRequests } from "$lib/api/sdk.gen";

const requests = createQuery({
  queryKey: ["brotherband-requests"],
  queryFn: () => listBrotherbandRequests({ query: { direction: "all" } }),
});
// → data.received: BrotherbandRequest[],  data.sent: BrotherbandRequest[]
```

### 9.4 Messaging with cursor pagination

The cursor is **opaque** — store it, send it back, never parse it. `nextCursor` is
`null` when there are no more pages.

```ts
import { createInfiniteQuery } from "@tanstack/svelte-query";
import { listMessagesWithBrother } from "$lib/api/sdk.gen";

export function messages(brotherId: string) {
  return createInfiniteQuery({
    queryKey: ["messages", brotherId],
    queryFn: ({ pageParam }) =>
      listMessagesWithBrother({
        path: { brotherId },
        query: { limit: 50, cursor: pageParam || undefined },
      }),
    initialPageParam: "",
    // server returns newest-first; nextCursor === null means done
    getNextPageParam: (last) => last.data?.nextCursor ?? undefined,
  });
}
```

Send a message (a conversation is created on the first message automatically):

```ts
import { sendMessageToBrother } from "$lib/api/sdk.gen";
await sendMessageToBrother({ path: { brotherId }, body: { body: text } });
// 403 brotherband.not_a_brother if you are not brothers
// 422 message.invalid_body if empty/too long (1–4000 chars)
```

`GET /v1/conversations` returns one row per brother (the conversation list); render
it as the inbox.

### 9.5 Media upload (3-step direct-to-R2 flow)

Images never pass through the API server. The flow is **always three steps**:

```
1. POST /v1/media/upload-url   → { uploadUrl, mediaKey, expiresAt }
2. PUT  {uploadUrl}  (the file, directly to Cloudflare R2)
3. PATCH /v1/me/avatar  OR  /v1/messages/{id}/attachment   { mediaKey }
```

Critical rules enforced by the server / R2:

- Allowed types: `image/jpeg`, `image/png`, `image/webp`. **Max 10 MB.** Validate
  client-side too — fail fast before the round-trip.
- The `PUT` to R2 **must** send `Content-Type` and `Content-Length` **equal to what
  you declared** in step 1 — R2 rejects a mismatch at the edge.
- The `PUT` to R2 is a **plain `fetch`**, not the generated client: it goes to
  Cloudflare, carries **no cookies and no CSRF header**, and must **not** use
  `credentials: 'include'`.
- The presign endpoint is rate-limited (`429 rate_limited`, ~20/min). Don't hammer it.

```svelte
<script lang="ts">
  import { createMutation } from '@tanstack/svelte-query';
  import { requestUploadUrl, attachMediaToMessage, updateMyAvatar } from '$lib/api/sdk.gen';

  const ALLOWED = ['image/jpeg', 'image/png', 'image/webp'];
  const MAX = 10 * 1024 * 1024;

  const upload = createMutation({
    mutationFn: async (args: { file: File; target: { messageId: string } | 'avatar' }) => {
      const { file, target } = args;
      if (!ALLOWED.includes(file.type)) throw new Error('Use JPEG, PNG or WebP.');
      if (file.size > MAX) throw new Error('Max 10 MB.');

      // 1. ask the API to presign
      const { data, error } = await requestUploadUrl({
        body: { contentType: file.type as never, contentLength: file.size },
      });
      if (error) throw error;
      const { uploadUrl, mediaKey } = data!;

      // 2. PUT the bytes straight to R2 — NO cookies, NO CSRF, exact headers
      const put = await fetch(uploadUrl, {
        method: 'PUT',
        headers: { 'Content-Type': file.type, 'Content-Length': String(file.size) },
        body: file,
      });
      if (!put.ok) throw new Error('Upload to storage failed.');

      // 3. tell the API to promote + record the key
      return target === 'avatar'
        ? updateMyAvatar({ body: { mediaKey } })
        : attachMediaToMessage({ path: { messageId: target.messageId }, body: { mediaKey } });
    },
  });
</script>

<input type="file" accept="image/jpeg,image/png,image/webp"
  onchange={(e) => {
    const f = e.currentTarget.files?.[0];
    if (f) $upload.mutate({ file: f, target: 'avatar' });
  }} />
```

After step 3, the server returns the updated resource; the stored media is then
served from the R2 CDN URL (`avatarUrl` on the profile, `attachments[].url` on a
message) — render those `url`s directly in `<img>`.

---

## 10. Suggested routing structure

```
src/routes/
├── +layout.ts             # configureApiClient(...) + session.refresh()
├── +layout.svelte         # QueryClientProvider
├── +page.svelte           # marketing / redirect to /app or /login
├── login/+page.svelte
├── register/+page.svelte
└── (app)/                 # protected group
    ├── +layout.ts         # auth guard (redirect to /login if 401)
    ├── +layout.svelte     # app shell / nav
    ├── +page.svelte       # conversations list (GET /v1/conversations)
    ├── brothers/+page.svelte
    ├── requests/+page.svelte
    ├── chat/[brotherId]/+page.svelte
    └── me/+page.svelte    # profile, status, avatar
```

---

## 11. Environment & CORS (dev and prod)

`web/.env`:

```
PUBLIC_API_BASE_URL=http://localhost:3000
```

**Dev cookie correctness (the classic footgun):** the frontend runs on
`http://localhost:3333`, the API on `http://localhost:3000`. These are different
_origins_ but the same _site_ (`localhost`), so `SameSite=Lax` cookies are sent on
these cross-origin-but-same-site requests when `credentials: 'include'` is set. It
works **only because**:

- the backend's `HTTP_ALLOWED_ORIGINS` includes `http://localhost:3333` (and `:3000`),
  and its CORS layer echoes the origin with `Access-Control-Allow-Credentials: true`;
- in `APP_ENV=development` the cookies are **not** `Secure`, so they survive plain
  `http`.

If you change the frontend dev port, add it to the backend's `HTTP_ALLOWED_ORIGINS`
or the browser will block the credentialed request.

**Production:** serve the web app and API under the same registrable domain
(e.g. `app.brotherband.app` + `api.brotherband.app`); set the backend
`HTTP_COOKIE_DOMAIN=.brotherband.app`, `APP_ENV=production` (cookies become `Secure`),
and add the web origin to `HTTP_ALLOWED_ORIGINS`. The `SameSite=Lax` + double-submit
CSRF model then holds without changes.

---

## 12. Endpoint → SDK function reference

With `operationId: true`, the generated SDK function name **is** the operationId.

| SDK function               | Method & path                                      | Notes                                |
| -------------------------- | -------------------------------------------------- | ------------------------------------ |
| `registerUser`             | `POST /v1/auth/register`                           | sets cookies; no CSRF                |
| `loginUser`                | `POST /v1/auth/login`                              | sets cookies; no CSRF                |
| `logoutUser`               | `POST /v1/auth/logout`                             | clears cookies; no CSRF              |
| `getMyProfile`             | `GET /v1/me`                                       | session probe                        |
| `updateMyStatus`           | `PATCH /v1/me/status`                              | CSRF                                 |
| `updateMyAvatar`           | `PATCH /v1/me/avatar`                              | CSRF; step 3 of upload               |
| `listBrothers`             | `GET /v1/brothers`                                 |                                      |
| `getBrotherProfile`        | `GET /v1/brothers/{brotherId}`                     |                                      |
| `cutBrotherband`           | `DELETE /v1/brothers/{brotherId}`                  | CSRF                                 |
| `listBrotherbandRequests`  | `GET /v1/brotherband-requests`                     | `?direction=received\|sent\|all`     |
| `sendBrotherbandRequest`   | `POST /v1/brotherband-requests/send/{recipientId}` | CSRF                                 |
| `acceptBrotherbandRequest` | `POST /v1/brotherband-requests/{requestId}/accept` | CSRF; **one-shot secret**            |
| `denyBrotherbandRequest`   | `POST /v1/brotherband-requests/{requestId}/deny`   | CSRF                                 |
| `listConversations`        | `GET /v1/conversations`                            | the inbox                            |
| `listMessagesWithBrother`  | `GET /v1/conversations/with/{brotherId}/messages`  | `?cursor=&limit=`                    |
| `sendMessageToBrother`     | `POST /v1/conversations/with/{brotherId}/messages` | CSRF                                 |
| `attachMediaToMessage`     | `PATCH /v1/messages/{messageId}/attachment`        | CSRF; step 3 of upload               |
| `requestUploadUrl`         | `POST /v1/media/upload-url`                        | CSRF; step 1 of upload; rate-limited |

`getLiveness` / `getReadiness` (`/healthz`, `/readyz`) exist but are infra probes —
the UI normally doesn't call them.

---

## 13. Gotchas specific to this API

1. **Never read `bb_session`.** It is httpOnly by design. To check auth, call
   `getMyProfile()` and treat `401` as logged-out.
2. **CSRF only on state-changing methods, and never on `/v1/auth/*`.** The interceptor
   in §5 already does the right thing; don't add CSRF to GETs or auth calls.
3. **The accept-request `requesterSecret` is shown once.** Capture it at `onSuccess`.
   No endpoint ever returns it again — there is no "show me their secret later".
4. **The R2 `PUT` is not an API call.** It goes to Cloudflare directly: plain `fetch`,
   no cookies, no CSRF, and the `Content-Type`/`Content-Length` must exactly match
   what you presigned or R2 rejects it.
5. **Cursors are opaque.** Persist and replay `nextCursor`; `null` = end. Don't decode
   it, don't construct one.
6. **Messaging requires an existing brotherhood.** `sendMessageToBrother` to a
   non-brother is `403 brotherband.not_a_brother` — gate the UI on the brothers list.
7. **`favorites` is exactly five.** The registration form must enforce this; the
   server rejects anything else with `422` and `details.field = "favorites[n]"`.
8. **Switch on `error.code`, not `error.message`.** Messages are human copy and may
   change; `code` is the stable contract (full registry in `api/openapi.yaml`).
9. **Surface `requestId`** on unexpected errors — it's the exact key to the server-side
   structured log line for that request.
10. **Regenerate the client (`npm run gen`) whenever the spec changes.** The `dev`/
    `build` scripts do this for you so types can never silently drift.

---

## Appendix: minimal end-to-end smoke (no UI)

```ts
import { configureApiClient } from "$lib/client";
import {
  registerUser,
  getMyProfile,
  listConversations,
} from "$lib/api/sdk.gen";

configureApiClient("http://localhost:3000");

await registerUser({
  body: {
    username: "web_demo",
    password: "Hunter2!Hunter2",
    birthdate: "1995-01-01",
    secret: "the owl flies at dusk",
    status: "hi",
    favorites: ["a", "b", "c", "d", "e"],
  },
});
console.log((await getMyProfile()).data); // the profile (cookies now set)
console.log((await listConversations()).data); // { conversations: [] } initially
```

This guide tracks `api/openapi.yaml`. If the two ever disagree, the spec — and the
running server it was audited against — wins; regenerate and update this document.
