import { test, expect } from "@playwright/test";
import { RECORD_ID_PATTERN, projectPathPattern } from "../../utils/ids";

test("record id pattern matches both spellings", () => {
  const plain = "641058e63b5a87103e1e088aa4e45a13";
  const uuid = "641058e6-3b5a-8710-3e1e-088aa4e45a13";
  const re = new RegExp(`^${RECORD_ID_PATTERN}$`);

  expect(re.test(plain)).toBe(true);
  expect(re.test(uuid)).toBe(true);
  // Not so loose that it matches a truncated or over-long id.
  expect(re.test(plain.slice(0, 31))).toBe(false);
  expect(re.test(`${plain}ff`)).toBe(false);
  expect(re.test("not-an-id")).toBe(false);

  // The engagements assertion: project id in either spelling, record id likewise.
  const route = projectPathPattern(plain, `engagements/${RECORD_ID_PATTERN}$`);
  expect(route.test(`https://x/projects/${uuid}/engagements/${uuid}`)).toBe(true);
  expect(route.test(`https://x/projects/${plain}/engagements/${plain}`)).toBe(true);
  expect(route.test(`https://x/projects/${uuid}/engagements`)).toBe(false);
});

test("project path pattern accepts both project id spellings", () => {
  const plain = "641058e63b5a87103e1e088aa4e45a13";
  const uuid = "641058e6-3b5a-8710-3e1e-088aa4e45a13";

  // Built from the UUID fixture, as the settings spec now does.
  const settings = projectPathPattern(uuid, "settings");
  expect(settings.test(`https://x/projects/${uuid}/settings`)).toBe(true);
  expect(settings.test(`https://x/projects/${plain}/settings`)).toBe(true);
  expect(settings.test("https://x/projects/other/settings")).toBe(false);

  // The Get Help path the Novera assertion checks.
  const chat = projectPathPattern(uuid, "support/chat/describe-issue");
  expect(chat.test(`https://x/projects/${plain}/support/chat/describe-issue`)).toBe(true);
  expect(chat.test(`https://x/projects/${uuid}/support/chat/create-case`)).toBe(false);
});

test("project path pattern ends at the suffix", () => {
  const uuid = "641058e6-3b5a-8710-3e1e-088aa4e45a13";
  const settings = projectPathPattern(uuid, "settings");

  // Exact, plus query and fragment.
  expect(settings.test(`https://x/projects/${uuid}/settings`)).toBe(true);
  expect(settings.test(`https://x/projects/${uuid}/settings?tab=users`)).toBe(true);
  expect(settings.test(`https://x/projects/${uuid}/settings#top`)).toBe(true);

  // Deeper paths must NOT match — the prefix-match bug this guards against.
  expect(settings.test(`https://x/projects/${uuid}/settings/users/42`)).toBe(false);
  expect(settings.test(`https://x/projects/${uuid}/settings-archive`)).toBe(false);

  // Composes with a suffix that already anchors itself ($), as several callers do.
  const anchored = projectPathPattern(uuid, "support$");
  expect(anchored.test(`https://x/projects/${uuid}/support`)).toBe(true);
  expect(anchored.test(`https://x/projects/${uuid}/support/cases`)).toBe(false);

  // And with a suffix matching its own query string.
  const withQuery = projectPathPattern(uuid, "support/cases\\?createdByMe=true");
  expect(withQuery.test(`https://x/projects/${uuid}/support/cases?createdByMe=true`)).toBe(true);
});
