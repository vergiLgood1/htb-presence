# Legal & Terms of Service notes

`htb-presence` is an unofficial, personal-use project. It is **not affiliated with,
endorsed by, or supported by Hack The Box (HTB)**.

This file is a maintainer checklist, **not legal advice**. It records what should be
reviewed before a public release and when it was last checked. Terms of Service documents
change; re-check periodically.

## Status

- [x] HTB's current public legal set reviewed against the points below.
- [x] Decision recorded: read-only polling of the internal API is **not** expressly
      authorised. Keep the disclaimer, stay read-only, and leave account risk with
      the user. Do not describe this project as compliant.

**Last reviewed:** 2026-09-28.

**Sources opened that day:**

- <https://www.hackthebox.com/legal> (index of PDFs; `/terms` redirects here).
- Acceptable Use Policy, effective 01 April 2026:
  <https://www.hackthebox.com/hubfs/Legal/AUP.pdf>
- End User Subscription Agreement and User Agreement PDFs linked from the same page
  (`EUSA.pdf`, `UA.pdf`). Those two are mostly subscription and content terms; the
  automation rules that matter here are in the AUP.

**What the AUP says, in short** (not a substitute for the PDF):

- Rule 8: do not use bots, scrapers, or automated tools to access or extract from
  the platform.
- Section 6.1: do not let an automated system access the Services except where a
  Service Agreement with Hack The Box expressly authorises it.
- Section 6.3: programmatic access is allowed where Hack The Box authorises it
  through its APIs, and even then you must not evade rate limits or harvest
  responses into a dataset.

An App Token is a credential the user can generate. The AUP does not say that
token makes internal v4 polling an authorised API. This file therefore does not
treat `htb-presence` as permitted. The practical consequence for this repo is
unchanged: read-only, conservative polling, no flag submission, a visible
disclaimer, and the user decides whether to run it.

## Why this matters

HTB has no public API for regular accounts. `htb-presence` talks to the same internal
v4/v5 endpoints HTB's web app uses, authenticated with a user-generated App Token. Using
those endpoints is governed by HTB's Terms of Service rather than an API agreement, so a
compliance decision is a human judgment call — it cannot be verified like an endpoint.

## Points to review in HTB's Terms of Service

- **Automated access / scraping** — is programmatic read-only polling with a personal App
  Token permitted, or treated as prohibited automation?
- **Rate limits / burden on the service** — HTB publishes no rate-limit contract for this
  API. Is a conservative poll interval plus backoff enough under "don't burden the
  service" style clauses?
- **Reverse engineering** — the endpoints are community reverse-engineered; does the ToS
  prohibit that?
- **App Tokens** — confirm the intended use of Profile → Settings → App Tokens, and
  whether personal tooling like this falls within it.
- **Redistribution / commercial use** — this project only renders the user's own activity
  locally and is not sold; confirm that is acceptable.
- **Account risk** — document for users that unofficial API use could, in principle,
  attract account action.

## What the code already does (mitigations, not legal cover)

- Read-only: it never performs actions on the user's behalf (no spawning, submitting or
  resetting).
- Conservative default poll interval (30s) with adaptive backoff, honoring `Retry-After`.
- Reads only the user's own data, and is never uploaded anywhere.
- The App Token is never logged in full (redacted, e.g. `eyJ0…`).

## Disclaimers

- This project is unofficial and is not affiliated with Hack The Box.
- Users are responsible for reviewing and complying with HTB's Terms of Service.
- Nothing in this file or repository is legal advice.

## References

- HTB Terms of Service — linked from the footer of <https://www.hackthebox.com/>.
- App Tokens — `app.hackthebox.com` → Profile → Settings → App Tokens.
