# Email Gateway

## Outbound
- Composed in BBS mail composer and sent via SMTP relay:
  - `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `FROM_DOMAIN`
  - or prefixed aliases: `WOLFBBS_SMTP_HOST`, `WOLFBBS_SMTP_PORT`, `WOLFBBS_SMTP_USER`, `WOLFBBS_SMTP_PASS`, `WOLFBBS_FROM_DOMAIN`
- Hard safety:
  - per-user send rate cap
  - max recipients per message
  - max message size
  - verified-account flag must be true before sending external recipients
  - global outbound disable toggle for incident response
- Runtime source of truth:
  - `/admin/gateways` values are applied at send time for outbound relay and reset-email delivery
  - if SMTP password is left blank in admin form, the existing stored secret is retained
- Audit log fields:
  - actor id, recipient count, subject, status, error, relay response, timestamp

## Password Reset Delivery
- `/reset/request` issues one-time reset tokens with expiration.
- If SMTP is configured and the account handle is an email-form handle, WolfBBS sends a reset link via email.
- Reset URL base:
  - `WOLFBBS_PUBLIC_BASE_URL` when set, otherwise the configured WolfBBS hostname/base URL.
- Delivery failures are logged server-side while API responses remain non-enumerating.
- Web reset requests are rate-limited per client address.

## Inbound (preferred optional)
- HTTP ingestion route is available:
  - `POST /mail/inbound`
  - Requires `X-Inbound-Token` matching `WOLFBBS_INBOUND_TOKEN`
  - Default dev token is accepted only from loopback/LAN callers; public exposure requires a custom token
  - JSON payload: `from`, `to`, `subject`, `body`, `raw_headers`
- Companion daemon (`cmd/wolfbbs-mailin`):
  - Receives inbound JSON on `/ingest`
  - Requires one of:
    - `X-Inbound-Token`
    - `Authorization: Bearer <token>`
    - `?token=<token>`
  - Applies sender-domain allowlist (`WOLFBBS_MAILIN_ALLOW_DOMAINS`)
  - Forwards to `/mail/inbound` with token auth
- Recipient mapping:
  - `user@example.com` -> `user`
  - `user+wolfbbs@example.com` -> `user`
- Inbound messages are stored as private mail using `mailbot` as sender.
- Anti-abuse:
  - shared-secret token required
  - plus-address mapping support
  - store raw headers/body preview for operator review

## Cloudflare Email Routing (free inbound)
- `deploy/cloudflare-email-worker/` holds a Worker that turns Email Routing deliveries into `/ingest` calls through a Cloudflare Tunnel, which gives callers receive-only `handle@<board domain>` addresses with no mail server. Setup is in that folder's README.

## Recovery Email
- Callers add a recovery email in `/settings`. It only counts once they confirm it from the emailed link (`/settings/recovery-email/confirm`, 24-hour single-use token).
- Password reset links go to the confirmed recovery email first, then to an email-form handle.
- The board host, its parent domain, and `WOLFBBS_RECOVERY_EMAIL_BLOCKED_DOMAINS` are refused as recovery addresses, and are never used as reset recipients.

## Admin Controls
- Global allowlist/denylist
- Per-user allow/disable outbound
- per-minute and per-day throttles
