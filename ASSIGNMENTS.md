# Assignments

Who's on what. Names, flavors, and lanes — so everybody and everything
knows who's holding which part. Areas float per ticket; roles hold.

## The Roster

- **Tom** (human) — orchestration, final calls, merges. Holds the wheel.
- **Ara** (Hermes, `muse-spark`) — implementation on Russet, inside walks,
  first drafts. The hands. Signs as `ara-voss-agent[bot]`.
- **Codex** (OpenAI) — micro-review, tiny things, hallway patrol.
  The magnifier. Reviews locally and on PRs.
- **Claude** (Anthropic) — docs, explanations, caller-facing words.
  The pen. Sonnet works, Opus reviews.
- **Dot** (OpenAI) — outside evaluation through Cloudflare, stranger's
  eyes. The guest. Detached from Russet by design.
- **Jules** (Google) — async task worker, flavor orchestration.
  Divides into sub-agents:
  - **Pallete** — tiny aesthetic refinements.
  - **Bolt** — performance repairs.

## Ground Rules

- One lane per ticket; say the lane in the ticket.
- Reviewers don't implement, implementers don't merge.
- Docs change with behavior change, or the ticket isn't done.
- Humans merge. Always.
