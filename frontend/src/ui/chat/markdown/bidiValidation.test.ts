import assert from "node:assert/strict";
import test from "node:test";
import { getTextDirection, getTextAlignClass, isRtlText, hasLtrText, splitBidiSegments } from "./bidi.ts";
import { parseMarkdown } from "./blockParser.ts";

test("Case 1: Arabic only", () => {
  const text = "هذا نص عربي بالكامل.";
  assert.equal(isRtlText(text), true);
  assert.equal(hasLtrText(text), false);
  assert.equal(getTextDirection(text), "rtl");
  assert.equal(getTextAlignClass(text), "text-right");
  const segs = splitBidiSegments(text);
  assert.equal(segs.length, 1);
  assert.equal(segs[0].isLtr, false);
});

test("Case 2: English only", () => {
  const text = "This is a normal English message with Markdown rendering.";
  assert.equal(isRtlText(text), false);
  assert.equal(hasLtrText(text), true);
  assert.equal(getTextDirection(text), "ltr");
  assert.equal(getTextAlignClass(text), "text-left");
});

test("Case 3: Arabic + English", () => {
  const text = "يمكن استخدام Angular للواجهة الأمامية و Python للواجهة الخلفية.";
  assert.equal(isRtlText(text), true);
  assert.equal(hasLtrText(text), true);
  assert.equal(getTextDirection(text), "rtl");
  assert.equal(getTextAlignClass(text), "text-right");
  const segs = splitBidiSegments(text);
  const ltr = segs.filter(s => s.isLtr).map(s => s.text);
  assert.deepEqual(ltr, ["Angular", "Python"]);
});

test("Case 4: Arabic with parentheses and OAuth / OIDC", () => {
  const text = "نستخدم OpenID Connect (OIDC) مع OAuth 2.0 لتسجيل الدخول.";
  assert.equal(isRtlText(text), true);
  assert.equal(getTextDirection(text), "rtl");
  assert.equal(getTextAlignClass(text), "text-right");
  const segs = splitBidiSegments(text);
  const ltr = segs.filter(s => s.isLtr).map(s => s.text);
  assert.deepEqual(ltr, ["OpenID Connect (OIDC)", "OAuth 2.0"]);
});

test("Case 5: Long English run with slashes and parentheses", () => {
  const text = "الخيار هو OAuth 2.0 / OpenID Connect (OIDC) للمصادقة.";
  assert.equal(isRtlText(text), true);
  assert.equal(getTextDirection(text), "rtl");
  const segs = splitBidiSegments(text);
  const ltr = segs.filter(s => s.isLtr).map(s => s.text);
  assert.deepEqual(ltr, ["OAuth 2.0 / OpenID Connect (OIDC)"]);
});

test("Case 6: Arabic list with English technical terms", () => {
  const md = `- الواجهة الأمامية Angular
- الواجهة الخلفية Python
- قاعدة البيانات PostgreSQL`;
  const blocks = parseMarkdown(md);
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0].type, "list");
  if (blocks[0].type === "list") {
    assert.equal(blocks[0].items.some(item => isRtlText(item.text)), true);
    for (const item of blocks[0].items) {
      assert.equal(isRtlText(item.text), true);
      assert.equal(getTextDirection(item.text), "rtl");
    }
  }
});

test("Case 7: Arabic heading", () => {
  const md = "## إدارة المستخدمين والصلاحيات";
  const blocks = parseMarkdown(md);
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0].type, "heading");
  if (blocks[0].type === "heading") {
    assert.equal(isRtlText(blocks[0].text), true);
    assert.equal(getTextDirection(blocks[0].text), "rtl");
    assert.equal(getTextAlignClass(blocks[0].text), "text-right");
  }
});

test("Case 8: Mixed table", () => {
  const md = `| العنصر | التقنية |
|---|---|
| الواجهة | Angular |
| Backend | Python / FastAPI |`;
  const blocks = parseMarkdown(md);
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0].type, "table");
  if (blocks[0].type === "table") {
    // Header cells
    assert.equal(getTextDirection(blocks[0].header[0]), "rtl");
    assert.equal(getTextDirection(blocks[0].header[1]), "rtl");
    // Row 1
    assert.equal(getTextDirection(blocks[0].rows[0][0]), "rtl");
    assert.equal(getTextDirection(blocks[0].rows[0][1]), "ltr");
    // Row 2
    assert.equal(getTextDirection(blocks[0].rows[1][0]), "ltr");
    assert.equal(getTextDirection(blocks[0].rows[1][1]), "ltr");
  }
});

test("Case 9: Technical inline code", () => {
  const text = "شغل الأمر `npm run build` ثم أعد تشغيل الخدمة.";
  assert.equal(isRtlText(text), true);
  assert.equal(getTextDirection(text), "rtl");
  assert.equal(getTextAlignClass(text), "text-right");
});

test("Case 10: Single Page Application - SPA", () => {
  const text = "يتم بناء الواجهة باستخدام Angular لتجربة Single Page Application - SPA سريعة.";
  const segs = splitBidiSegments(text);
  const ltr = segs.filter(s => s.isLtr).map(s => s.text);
  assert.deepEqual(ltr, ["Angular", "Single Page Application - SPA"]);
});

test("Case 11: LDAP / Active Directory", () => {
  const text = "ندعم الربط مع LDAP / Active Directory لإدارة الهويات.";
  const segs = splitBidiSegments(text);
  const ltr = segs.filter(s => s.isLtr).map(s => s.text);
  assert.deepEqual(ltr, ["LDAP / Active Directory"]);
});

test("Case 12: Backend API and Django REST Framework", () => {
  const text = "نستخدم Django REST Framework لبناء Backend API موثوق.";
  const segs = splitBidiSegments(text);
  const ltr = segs.filter(s => s.isLtr).map(s => s.text);
  assert.deepEqual(ltr, ["Django REST Framework", "Backend API"]);
});

test("Case 13: Bold English inside Arabic with parentheses", () => {
  const text = "**Angular** هو إطار عمل للواجهة الأمامية **(Frontend / Client-side UI)**.";
  assert.equal(isRtlText(text), true);
  assert.equal(getTextDirection(text), "rtl");
  assert.equal(getTextAlignClass(text), "text-right");
});

test("Case 14: Bold OAuth 2.0 / OpenID Connect (OIDC)", () => {
  const text = "نستخدم **OAuth 2.0 / OpenID Connect (OIDC)** لتسجيل الدخول.";
  assert.equal(isRtlText(text), true);
  assert.equal(getTextDirection(text), "rtl");
});

test("Case 15: HTTPS URL in Arabic text", () => {
  const text = "راجع الرابط https://example.com/docs/api لمزيد من التفاصيل.";
  assert.equal(isRtlText(text), true);
  assert.equal(getTextDirection(text), "rtl");
  assert.equal(getTextAlignClass(text), "text-right");
});
