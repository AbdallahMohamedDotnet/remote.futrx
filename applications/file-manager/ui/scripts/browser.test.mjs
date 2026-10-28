import assert from "node:assert/strict";
import test from "node:test";

const base = process.env.FUTRX_FILE_MANAGER_URL;
test(
  "browse, search, download, error recovery and mobile layout",
  { skip: !base },
  async () => {
    const { chromium } = await import("playwright");
    const browser = await chromium.launch({
      headless: true,
      args: ["--no-sandbox"],
    });
    try {
      const page = await browser.newPage({
        viewport: { width: 390, height: 844 },
      });
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.goto(base);
      await page.getByRole("button", { name: "▸ src", exact: true }).click();
      await page.getByRole("button", { name: "app.go", exact: true }).waitFor();
      await page.getByLabel("Search all files", { exact: true }).fill("app");
      await page.waitForFunction(
        () => document.querySelector("#status").textContent === "1 result",
      );
      assert.equal(
        await page.locator("button.open").filter({ hasText: "app.go" }).count(),
        1,
      );
      const [download] = await Promise.all([
        page.waitForEvent("download"),
        page
          .getByRole("link", { name: "Download app.go", exact: true })
          .click(),
      ]);
      assert.equal(download.suggestedFilename(), "app.go");
      await page.getByLabel("Search all files", { exact: true }).fill("");
      await page.getByRole("button", { name: "▸ src", exact: true }).waitFor();
      const [archive] = await Promise.all([
        page.waitForEvent("download"),
        page
          .getByRole("link", { name: "Download src as ZIP", exact: true })
          .click(),
      ]);
      assert.equal(archive.suggestedFilename(), "src.zip");
      await page.route("**/api/list", (route) =>
        route.fulfill({ status: 503, body: "Unavailable" }),
      );
      await page.getByRole("button", { name: "Refresh", exact: true }).click();
      await page.waitForFunction(
        () => document.querySelector("#status").className === "error",
      );
      assert.match(
        await page.locator("#status").textContent(),
        /Unable to load/,
      );
      await page.unroute("**/api/list");
      await page.getByRole("button", { name: "Refresh", exact: true }).click();
      await page.getByRole("button", { name: "▸ src", exact: true }).waitFor();
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
        true,
      );
      assert.deepEqual(errors, []);
    } finally {
      await browser.close();
    }
  },
);

test(
  "application drawer bridges only its iframe and disposes cleanly",
  { skip: !base },
  async () => {
    const { chromium } = await import("playwright");
    const { readFile } = await import("node:fs/promises");
    const browser = await chromium.launch({
      headless: true,
      args: ["--no-sandbox"],
    });
    try {
      const page = await browser.newPage();
      await page.route("http://remote.test/**", async (route) => {
        const pathname = new URL(route.request().url()).pathname;
        if (pathname === "/main.js" || pathname === "/workspacePath.js") {
          await route.fulfill({
            contentType: "text/javascript",
            body: await readFile(
              new URL(`.${pathname}`, import.meta.url),
              "utf8",
            ),
          });
          return;
        }
        await route.fulfill({
          contentType: "text/html",
          body: `<!doctype html><html style="--fg: 12 13 14;--fg-muted: 80 80 80;--bg-surface-rgb: 240 241 242;--bg-inset-rgb: 230 231 232;--accent-blue: 20 90 200;--accent-red: 200 20 20;--line: #ccc"><body><div id="drawer" style="width:560px;height:600px"></div><script type="module">
    import activate from '/main.js';
    window.opened=[];
    activate({application:{web:{subdomain:'files'}},files:{open:request=>window.opened.push(request)},ui:{registerDrawer:drawer=>{
     window.dispose=drawer.mount(document.querySelector('#drawer'),{chatId:'c1',projectId:'p1',cwd:'/var/lib/remote/projects/alpha/workspace',setStatus:()=>{}});
    }}});
   </script></body></html>`,
        });
      });
      await page.route("http://files--alpha.remote.test/**", async (route) => {
        const url = new URL(route.request().url());
        const response = await route.fetch({
          url: base + url.pathname + url.search,
        });
        await route.fulfill({ response });
      });
      await page.goto("http://remote.test/");
      const frame = page.frameLocator("iframe");
      await frame.getByRole("button", { name: "▸ src", exact: true }).click();
      await frame.getByRole("button", { name: "app.go", exact: true }).click();
      await page.waitForFunction(() => window.opened.length === 1);
      assert.deepEqual(await page.evaluate(() => window.opened[0]), {
        chatId: "c1",
        projectId: "p1",
        cwd: "/var/lib/remote/projects/alpha/workspace",
        path: "/workspace/src/app.go",
      });
      assert.equal(
        await frame
          .locator("body")
          .evaluate((body) => getComputedStyle(body).color),
        "rgb(12, 13, 14)",
      );
      await page.evaluate(() => {
        const source = document.querySelector("iframe").contentWindow;
        const data = {
          source: "remote-file-manager",
          type: "open",
          path: "src/app.go",
        };
        window.dispatchEvent(
          new MessageEvent("message", {
            source,
            origin: "http://evil.test",
            data,
          }),
        );
        window.dispatchEvent(
          new MessageEvent("message", {
            source,
            origin: "http://files--alpha.remote.test",
            data: { ...data, path: "../secret" },
          }),
        );
      });
      assert.equal(await page.evaluate(() => window.opened.length), 1);
      await page.evaluate(() => window.dispose());
      assert.equal(await page.locator("iframe").count(), 0);
    } finally {
      await browser.close();
    }
  },
);
