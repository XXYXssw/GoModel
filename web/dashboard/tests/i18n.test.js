import assert from "node:assert/strict";
import test from "node:test";
import { dictionaries } from "../src/lib/i18n/dictionaries/index.js";
import { setLocale, getLocale, t } from "../src/lib/i18n/core.js";

test("en and zh dictionaries expose the same key set", () => {
  const enKeys = Object.keys(dictionaries.en).sort();
  const zhKeys = Object.keys(dictionaries.zh).sort();
  assert.deepEqual(zhKeys, enKeys, "zh must mirror en exactly (and vice versa)");
});

test("t returns the current locale string", () => {
  setLocale("zh");
  assert.equal(t("common.cancel"), "取消");
  setLocale("en");
  assert.equal(t("common.cancel"), "Cancel");
});

test("t falls back to English and then to the key itself", () => {
  setLocale("zh");
  // Present in en but not yet translated in zh would still fall back to en.
  assert.equal(t("nav.models"), "模型"); // zh has it
  assert.equal(t("definitely.not.a.key"), "definitely.not.a.key");
});

test("t interpolates {placeholders}", () => {
  setLocale("en");
  assert.equal(t("models.count", { count: 3 }), "3 models");
  assert.equal(t("vm.allModelsIn", { name: "openai" }), "All models in openai");
});

test("setLocale ignores unsupported locales", () => {
  setLocale("fr");
  assert.equal(getLocale(), "en"); // unchanged from the last supported value
});
