// /workspace/scripts/browser.mjs — generic authenticated-browser CLI for
// every futrx project. Reads /workspace/.agents/browser-auth.json to figure
// out which cookie to attach for the requested URL, then drives Playwright.
//
// USAGE
//   node /workspace/scripts/browser.mjs screenshot <url> [--out <path>] [--full]
//   node /workspace/scripts/browser.mjs record     <url> [--duration <ms>] [--out <path>]
//
//   --out         override the output file path (default /workspace/.browser/<ts>.<ext>)
//   --full        full-page screenshot (default: viewport only)
//   --duration    record duration in ms (default 5000)
//
// CONFIG (/workspace/.agents/browser-auth.json)
//   {
//     "<request-host>": {
//       "cookies": [
//         { "name": "<cookie-name>", "domain": "<cookie-domain>", "secret": "<ENV_VAR>",
//           "path": "/", "httpOnly": true, "secure": true, "sameSite": "None" }
//       ]
//     }
//   }
//
// Match rules for "<request-host>":
//   - exact host match (e.g. "app.graphixy.ai")
//   - wildcard "*.example.com" matches any sub.example.com
//   - "default" matches anything not otherwise listed (skip if you don't want
//     fallback)
//
// MISSING-COOKIE BEHAVIOUR
//   If the URL's host isn't in the config, or the named secret isn't set in
//   the environment, the script exits with a clear instruction telling the
//   agent which entry to add and which secret to ask the user to paste.
//   No silent retries, no logged-out fallback.
//
// OUTPUT — written to /workspace/.browser/ (override with $BROWSER_OUT_DIR).
//   Path is printed on stdout so callers can `Read` the file.

import { mkdir, readFile, rename, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const CONFIG_PATH = process.env.BROWSER_AUTH_CONFIG || '/workspace/.agents/browser-auth.json';
const OUT_DIR = process.env.BROWSER_OUT_DIR || '/workspace/.browser';
const VIEWPORT = { width: 1280, height: 720 };

function die(code, msg) {
  process.stderr.write(msg + '\n');
  process.exit(code);
}

function flag(args, name) {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : undefined;
}
function hasFlag(args, name) {
  return args.includes(`--${name}`);
}

function usage() {
  die(2, [
    'usage:',
    '  node /workspace/scripts/browser.mjs screenshot <url> [--out <path>] [--full]',
    '  node /workspace/scripts/browser.mjs record     <url> [--duration <ms>] [--out <path>]',
    '',
    `config: ${CONFIG_PATH}`,
    `output: ${OUT_DIR} (override with $BROWSER_OUT_DIR)`,
  ].join('\n'));
}

const args = process.argv.slice(2);
const [cmd, urlArg, ...rest] = args;
if (!cmd || !urlArg) usage();
if (!['screenshot', 'record'].includes(cmd)) usage();

let url;
try {
  url = new URL(urlArg);
} catch {
  die(2, `not a valid URL: ${urlArg}`);
}

// --- Load Playwright -----------------------------------------------------
let chromium;
try {
  ({ chromium } = await import('playwright'));
} catch {
  die(4, [
    'playwright is not installed in this workspace.',
    '',
    'Install once (downloads ~200MB the first time, cached afterwards):',
    '  cd /workspace && npm init -y >/dev/null && npm install --save-dev playwright',
    '  npx playwright install chromium',
    '',
    'Future invocations of this script will work without setup.',
  ].join('\n'));
}

// --- Load + match config -------------------------------------------------
let config = {};
if (existsSync(CONFIG_PATH)) {
  try {
    config = JSON.parse(await readFile(CONFIG_PATH, 'utf8'));
  } catch (err) {
    die(5, `failed to parse ${CONFIG_PATH}: ${err.message}`);
  }
} else {
  // Create an empty config so the agent has a file to add to.
  await mkdir(dirname(CONFIG_PATH), { recursive: true });
  await writeFile(CONFIG_PATH, '{}\n');
}

function pickEntry(host) {
  if (config[host]) return config[host];
  for (const key of Object.keys(config)) {
    if (key.startsWith('*.')) {
      const suffix = key.slice(1); // ".example.com"
      if (host.endsWith(suffix) && host !== suffix.slice(1)) return config[key];
    }
  }
  return config.default;
}

const entry = pickEntry(url.host);
if (!entry) {
  die(6, [
    `no auth registered for host ${url.host}.`,
    '',
    `Add an entry to ${CONFIG_PATH}:`,
    '',
    JSON.stringify(
      {
        ...config,
        [url.host]: {
          cookies: [
            {
              name: '<the-cookie-name>',
              domain: url.host,
              secret: `${url.host.replace(/[^A-Z0-9]/gi, '_').toUpperCase()}_COOKIE`,
              path: '/',
              httpOnly: true,
              secure: true,
              sameSite: 'None',
            },
          ],
        },
      },
      null,
      2,
    ),
    '',
    'Then ask the user to paste the cookie value into Containers → Secrets',
    'under the secret name you chose.',
  ].join('\n'));
}

const cookies = (entry.cookies || []).map((c) => {
  if (!c.secret) {
    die(7, `${CONFIG_PATH}: cookie for ${url.host} is missing "secret" (the env-var name holding the cookie value)`);
  }
  const value = process.env[c.secret];
  if (!value) {
    die(8, [
      `secret ${c.secret} is not set in the environment.`,
      '',
      `Ask the user to add it via the project Containers → Secrets UI.`,
      `Tell them which cookie to copy: ${c.name} from ${c.domain}.`,
    ].join('\n'));
  }
  return {
    name: c.name,
    value,
    domain: c.domain || url.host,
    path: c.path || '/',
    httpOnly: c.httpOnly !== false,
    secure: c.secure !== false,
    sameSite: c.sameSite || 'None',
    ...(c.expires != null ? { expires: c.expires } : {}),
  };
});

// --- Launch + drive Playwright ------------------------------------------
await mkdir(OUT_DIR, { recursive: true });

const launchOpts = { headless: true };
const contextOpts = { viewport: VIEWPORT };
if (cmd === 'record') {
  contextOpts.recordVideo = { dir: OUT_DIR, size: VIEWPORT };
}

const browser = await chromium.launch(launchOpts);
const context = await browser.newContext(contextOpts);
if (cookies.length) await context.addCookies(cookies);
const page = await context.newPage();

let exitCode = 0;
let recordOverride;
try {
  if (cmd === 'screenshot') {
    await page.goto(url.toString(), { waitUntil: 'networkidle', timeout: 30_000 });
    const out = resolve(flag(rest, 'out') || `${OUT_DIR}/screenshot-${ts()}.png`);
    await mkdir(dirname(out), { recursive: true });
    await page.screenshot({ path: out, fullPage: hasFlag(rest, 'full') });
    process.stdout.write(out + '\n');
  } else if (cmd === 'record') {
    const duration = parseInt(flag(rest, 'duration') || '5000', 10);
    await page.goto(url.toString(), { timeout: 30_000 });
    await page.waitForTimeout(duration);
    recordOverride = flag(rest, 'out');
  }
} catch (err) {
  process.stderr.write(`error: ${err.message}\n`);
  exitCode = 1;
} finally {
  const video = cmd === 'record' ? page.video() : null;
  await page.close();
  await context.close();
  await browser.close();
  if (cmd === 'record' && video) {
    const defaultPath = await video.path();
    if (recordOverride) {
      const target = resolve(recordOverride);
      await mkdir(dirname(target), { recursive: true });
      await rename(defaultPath, target);
      process.stdout.write(target + '\n');
    } else {
      process.stdout.write(defaultPath + '\n');
    }
  }
}

process.exit(exitCode);

function ts() {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
}
