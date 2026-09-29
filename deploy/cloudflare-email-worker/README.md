# Free BBS email with Cloudflare

This gives every caller a receive-only address, `handle@<your board domain>`, without running a mail server. It
also covers outgoing mail (password resets, recovery-email confirmations) through a free SMTP provider.

```
sender ──> Cloudflare Email Routing ──> this Worker ──HTTPS──> Tunnel ──> wolfbbs-mailin :8091 ──> caller's BBS mailbox
BBS ──SMTP 587──> free relay (Brevo, Resend, ...) ──> caller's personal inbox
```

Everything here is on free plans. Running your own MTA from home doesn't work well: residential IPs are on spam
blocklists, ISPs block port 25, and a Cloudflare Tunnel doesn't carry inbound SMTP.

## Use a subdomain if the main domain already has email

If the apex domain already receives mail somewhere else (iCloud, Google, Fastmail), **don't enable Email Routing on
the apex**. That would replace its MX records. Enable it only on the board's subdomain (for example
`reso.example.com`). Cloudflare adds MX and SPF records for that subdomain only.

## 1. Publish mailin through the tunnel

In Zero Trust, go to **Networks → Tunnels → your tunnel → Published application routes** and add a route:

- Hostname: `mail-in.<your domain>`
- Service: `http://localhost:8091`

If Access protects your hostnames, give this one an Access application whose policy is **Service Auth** with a new
**service token**. Keep the token's Client ID and Secret for step 3. Only the Worker can then reach mailin, and people
using a browser can't.

## 2. Turn on Email Routing for the board subdomain

In the Cloudflare dashboard, open your domain and go to **Email → Email Routing**. Enable it for the subdomain, then
**Routing rules → Catch-all address → Send to a Worker** and pick `wolfbbs-inbound-mail` after you deploy it in step 3.

## 3. Deploy the Worker

```bash
cd deploy/cloudflare-email-worker
npm install
# set WOLFBBS_INGEST_URL in wrangler.toml to https://mail-in.<your domain>/ingest
npx wrangler secret put WOLFBBS_INBOUND_TOKEN      # same value as WOLFBBS_INBOUND_TOKEN in <prefix>/.env
npx wrangler secret put CF_ACCESS_CLIENT_ID        # only if you added Access in step 1
npx wrangler secret put CF_ACCESS_CLIENT_SECRET
npx wrangler deploy
```

Behaviour:
- mail to a handle that exists goes into that caller's BBS mailbox, from `mailbot`
- mail to an unknown handle bounces with "No such mailbox on this BBS."
- attachments aren't delivered (their names are listed), and bodies over 200,000 characters are truncated
- if the BBS is offline, the Worker reports a temporary failure so the sender retries later

## 4. Outgoing mail on a free SMTP relay

Pick a provider with a free tier and SMTP on port 587 (STARTTLS). Brevo and Resend both work. Verify the board
subdomain with them by adding the DNS records they give you in Cloudflare. Then set these in `/admin/gateways`, or in
`<prefix>/.env` followed by `bash install.sh --start`:

```
WOLFBBS_SMTP_HOST=<provider smtp host>
WOLFBBS_SMTP_PORT=587
WOLFBBS_SMTP_USER=<provider smtp login>
WOLFBBS_SMTP_PASS=<provider smtp key>
WOLFBBS_FROM_DOMAIN=<board subdomain>
```

WolfBBS's SMTP client uses STARTTLS. Providers that only offer implicit TLS on port 465 won't work yet.

## Recovery emails

Callers set a recovery email in `/settings`, and it only counts once they click the confirmation link. Password
resets go to that confirmed address. The board's own domain and its parent domain are always refused, and
`WOLFBBS_RECOVERY_EMAIL_BLOCKED_DOMAINS` (comma-separated) adds more, so nobody can lock their recovery behind the
BBS mailbox they'd lose along with their account.
