import test from "node:test";
import assert from "node:assert/strict";
import { usageRangeService } from "./usageRangeService.ts";

const NOW = Date.parse("2026-08-17T15:42:11.000Z");
const DAY_MS = 24 * 60 * 60 * 1000;

test("bounds a day in UTC", () => {
  assert.equal(usageRangeService.toDateInputValue(usageRangeService.startOfUtcDay(NOW)), "2026-08-17");
  assert.equal(new Date(usageRangeService.startOfUtcDay(NOW)).toISOString(), "2026-08-17T00:00:00.000Z");
  assert.equal(new Date(usageRangeService.endOfUtcDay(NOW)).toISOString(), "2026-08-17T23:59:59.999Z");
});

test("resolves each preset to an inclusive window", () => {
  const week = usageRangeService.forPreset("7d", NOW);
  assert.equal(usageRangeService.toDateInputValue(week.from), "2026-08-11");
  assert.equal(usageRangeService.days(week), 7);

  const month30 = usageRangeService.forPreset("30d", NOW);
  assert.equal(usageRangeService.toDateInputValue(month30.from), "2026-07-19");
  assert.equal(usageRangeService.days(month30), 30);

  const thisMonth = usageRangeService.forPreset("month", NOW);
  assert.equal(usageRangeService.toDateInputValue(thisMonth.from), "2026-08-01");
  assert.equal(usageRangeService.days(thisMonth), 17);

  for (const range of [week, month30, thisMonth]) {
    assert.equal(range.to, usageRangeService.endOfUtcDay(NOW));
    assert.ok(range.from < range.to);
  }
});

test("this month starts on the first even when the month just rolled over", () => {
  const firstOfMonth = Date.parse("2026-09-01T00:05:00.000Z");
  const range = usageRangeService.forPreset("month", firstOfMonth);
  assert.equal(usageRangeService.toDateInputValue(range.from), "2026-09-01");
  assert.equal(usageRangeService.days(range), 1);
});

test("parses and rejects date input values", () => {
  assert.equal(usageRangeService.fromDateInputValue("2026-08-17"), Date.parse("2026-08-17T00:00:00.000Z"));
  for (const bad of ["", "17/08/2026", "2026-8-1", "not a date"]) {
    assert.equal(usageRangeService.fromDateInputValue(bad), null, bad);
  }
});

test("builds a custom range and swaps reversed inputs", () => {
  const current = usageRangeService.forPreset("30d", NOW);

  const forward = usageRangeService.fromDates(current, "2026-08-01", "2026-08-03");
  assert.equal(forward.preset, "custom");
  assert.deepEqual(usageRangeService.labels(forward), { fromDate: "2026-08-01", toDate: "2026-08-03" });
  assert.equal(usageRangeService.days(forward), 3);

  const reversed = usageRangeService.fromDates(current, "2026-08-03", "2026-08-01");
  assert.deepEqual(usageRangeService.labels(reversed), { fromDate: "2026-08-01", toDate: "2026-08-03" });
});

test("a single custom day is one full day, not an empty window", () => {
  const range = usageRangeService.fromDates(usageRangeService.forPreset("7d", NOW), "2026-08-17", "2026-08-17");
  assert.equal(usageRangeService.days(range), 1);
  assert.equal(range.to - range.from, DAY_MS - 1);
});

test("malformed custom input keeps the current range", () => {
  const current = usageRangeService.forPreset("7d", NOW);
  assert.deepEqual(usageRangeService.fromDates(current, "nope", "2026-08-03"), current);
  assert.deepEqual(usageRangeService.fromDates(current, "2026-08-03", ""), current);
});
