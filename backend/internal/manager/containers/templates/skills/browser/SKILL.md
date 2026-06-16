---
name: browser
description: "Drive a real web browser the user logs into — open pages, read content, search, fill forms, click, and act in the user's authenticated sessions (social media, dashboards, any site). Use when a task needs a live browser: 'go to <site>', 'search X on <site>', 'log into my <account> and do Y', 'check my messages/notifications', or automating a site that needs a real logged-in session. NOT for previewing this project's own dev server (that's the Browser drawer) and NOT for a one-off screenshot of a public URL (use scripts/browser.mjs)."
---

# Browser

You can drive a **real Chromium running inside this container** that the user
can watch and log into. With this skill active you have `browser_*` MCP tools
attached over CDP to that **live, shared session** — the same browser the user
sees in the Browser pane — so you inherit its cookies and logins.

## How to work
Go **step by step**, never blind:
1. `browser_navigate` to a URL.
2. `browser_snapshot` to read the page as an accessibility tree — this is your
   primary way to "see". It's cheaper and more reliable than screenshots, and
   it gives you the element refs the action tools need.
3. Act: `browser_click`, `browser_type`, `browser_fill_form`,
   `browser_press_key`, `browser_select_option`, `browser_hover`.
4. `browser_snapshot` again to confirm the result, and adapt.

Use `browser_take_screenshot` only when you need to *show* the user a picture;
use `browser_tabs`, `browser_navigate_back`, and `browser_wait_for` to manage
navigation. Don't guess selectors — read a snapshot first.

## Logging in (you never type passwords)
This browser exits via the container's datacenter IP, so strict providers
(Google, X) may show a "verify it's you" challenge. For anything that needs a
login you can't complete yourself:
1. Ask the user to open the **Browser** pane and log in by hand — they handle
   the password and any 2FA.
2. Their login persists; then drive the `browser_*` tools against that session.

Never enter the user's credentials yourself.

## Write policy
Reading (search, timelines, messages) is fine on your own. Before any **public
or irreversible write** — posting, replying, DMing, following, buying, or
changing settings — **say exactly what you're about to do and get the user's
confirmation first.** They can watch and stop you in the Browser pane.

## If the tools don't respond
The live browser may not be running yet. Ask the user to open the **Browser**
pane (that starts the session), then retry.
