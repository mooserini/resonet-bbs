// WolfBBS inbound mail bridge for Cloudflare Email Routing.
//
// Cloudflare receives mail for handle@<your board domain>, runs this Worker,
// and the Worker posts a small JSON payload to wolfbbs-mailin (/ingest)
// through your Cloudflare Tunnel. WolfBBS drops it into that caller's private
// mailbox, sent by "mailbot". Unknown handles bounce back to the sender.
//
// Required secrets (wrangler secret put <NAME>):
//   WOLFBBS_INBOUND_TOKEN   same value as WOLFBBS_INBOUND_TOKEN in the BBS .env
// Optional secrets, when the ingest hostname sits behind Cloudflare Access:
//   CF_ACCESS_CLIENT_ID, CF_ACCESS_CLIENT_SECRET   an Access service token
// Required var (wrangler.toml [vars]):
//   WOLFBBS_INGEST_URL      e.g. https://mail-in.example.com/ingest

import PostalMime from "postal-mime";

const MAX_BODY_CHARS = 200_000; // mailin accepts up to 1 MiB of JSON
const MAX_HEADER_CHARS = 16_000;

export default {
  async email(message, env) {
    const raw = await new Response(message.raw).arrayBuffer();
    const parsed = await PostalMime.parse(raw);

    let body = parsed.text || stripHtml(parsed.html || "");
    if (parsed.attachments && parsed.attachments.length > 0) {
      const names = parsed.attachments.map((a) => a.filename || a.mimeType).join(", ");
      body += `\n\n[${parsed.attachments.length} attachment(s) not delivered to the BBS: ${names}]`;
    }
    if (body.length > MAX_BODY_CHARS) {
      body = body.slice(0, MAX_BODY_CHARS) + "\n\n[message truncated]";
    }
    const rawHeaders = (parsed.headers || [])
      .map((h) => `${h.key}: ${h.value}`)
      .join("\n")
      .slice(0, MAX_HEADER_CHARS);

    const headers = {
      "Content-Type": "application/json",
      "X-Inbound-Token": env.WOLFBBS_INBOUND_TOKEN,
    };
    if (env.CF_ACCESS_CLIENT_ID && env.CF_ACCESS_CLIENT_SECRET) {
      headers["CF-Access-Client-Id"] = env.CF_ACCESS_CLIENT_ID;
      headers["CF-Access-Client-Secret"] = env.CF_ACCESS_CLIENT_SECRET;
    }

    let res;
    try {
      res = await fetch(env.WOLFBBS_INGEST_URL, {
        method: "POST",
        headers,
        body: JSON.stringify({
          from: message.from,
          to: message.to,
          subject: parsed.subject || "(no subject)",
          body,
          raw_headers: rawHeaders,
        }),
      });
    } catch (err) {
      // Throwing makes Cloudflare report a temporary failure so the sender retries.
      throw new Error(`BBS unreachable: ${err}`);
    }

    if (res.status === 404) {
      message.setReject("No such mailbox on this BBS.");
      return;
    }
    if (res.status === 400 || res.status === 403) {
      message.setReject("Message refused by the BBS.");
      return;
    }
    if (!res.ok) {
      throw new Error(`BBS ingest returned ${res.status}`);
    }
  },
};

function stripHtml(html) {
  return html
    .replace(/<(script|style)[\s\S]*?<\/\1>/gi, "")
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/p>/gi, "\n\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .trim();
}
