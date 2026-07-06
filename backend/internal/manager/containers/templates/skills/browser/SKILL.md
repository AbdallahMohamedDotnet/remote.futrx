---
name: browser
description: "Drive a real web browser the user logs into - open pages, read content, search, fill forms, click, and act in the user's authenticated sessions (social media, dashboards, any site). Use when a task needs a live browser: 'go to <site>', 'search X on <site>', 'log into my <account> and do Y', 'check my messages/notifications', or automating a site that needs a real logged-in session. NOT for previewing this project's own dev server (that's the Browser drawer) and NOT for a one-off screenshot of a public URL (use scripts/browser.mjs)."
---

# Browser

You can drive a real Chromium running inside this container. With this skill
active, the browser core starts automatically and you have `browser_*` MCP
tools attached over CDP to the live shared session. If the user opens the
Browser pane, they see the same browser and can log in by hand; you inherit
that session's cookies without handling credentials.

## How to work
Use a hybrid perception loop:

1. `browser_navigate`, then `browser_snapshot`. The snapshot is the default
   way to read structure, text, roles, and element refs.
2. Take a screenshot when the task is visual: layout verification, charts,
   maps, canvas, image content, custom widgets, anything opaque or missing
   from the snapshot, and after actions with visible consequences.
3. Act through refs when the snapshot gives reliable targets. Use
   screenshot-grounded coordinate interaction when the DOM is misleading.
4. Observe again after each action with `browser_snapshot`,
   `browser_take_screenshot`, or `browser_wait_for`. Do not assume an action
   worked.

Keep screenshot cost bounded: viewport screenshots by default, JPEG/moderate
quality when options are available, full-page screenshots only when needed.
The viewport is 1366x768 and maps 1:1 to OS-level fallback coordinates.

## Pacing

Wait for navigation/load states before acting. Use `browser_wait_for` when
content is dynamic. Take one meaningful action, then observe. Avoid blind
multi-action bursts, especially on authenticated sites.

## Logging in

This browser exits via the container's datacenter IP, so strict providers may
show verification challenges. If a task needs a login or challenge response:

1. Ask the user to open the Browser pane and log in by hand.
2. Wait until they say the login is complete.
3. Continue with the `browser_*` tools against the shared session.

Never type the user's credentials yourself.

## OS-level input fallback

Use the `browser_*` tools first. If a site visibly swallows or rejects CDP
input, fall back to X-server input with coordinates from a screenshot:

```sh
sh /workspace/.browser-gui/human-input.sh move 640 420
sh /workspace/.browser-gui/human-input.sh click 640 420
sh /workspace/.browser-gui/human-input.sh type "text to type"
sh /workspace/.browser-gui/human-input.sh key Tab Return
sh /workspace/.browser-gui/human-input.sh scroll -3
```

Coordinates map to the browser window at 0,0 on the 1366x768 display.

## Write policy

Reading (search, timelines, messages) is fine on your own. Before any public
or irreversible write - posting, replying, DMing, following, buying, or
changing settings - say exactly what you're about to do and get the user's
confirmation first. They can watch and stop you in the Browser pane.
