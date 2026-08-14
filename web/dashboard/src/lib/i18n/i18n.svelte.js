// Lightweight i18n: a rune-backed locale store plus a dictionary lookup with
// {placeholder} interpolation. There is no pluralization or ICU machinery —
// the dictionaries are flat key → string maps (en/zh). Locale choice persists
// in localStorage and follows the browser language on first visit.
//
// Components import t() from this module (reactive via the `locale` rune);
// plain logic modules loaded by node:test import the rune-free core instead.
import { readStored, writeStored } from "$lib/utils/storage.js";
import { getLocale, setLocale as setCoreLocale, t as coreT } from "./core.js";
import { dictionaries } from "./dictionaries/index.js";

const STORAGE_KEY = "gomodel.locale";
const SUPPORTED = Object.keys(dictionaries);

class I18n {
  locale = $state("en");

  init() {
    const stored = readStored(STORAGE_KEY);
    this.locale = SUPPORTED.includes(stored) ? stored : this.#detect();
    this.#apply();
  }

  #detect() {
    if (typeof navigator === "undefined") return "en";
    const lang = String(navigator.language || "").toLowerCase();
    return lang.startsWith("zh") ? "zh" : "en";
  }

  setLocale(locale) {
    if (!SUPPORTED.includes(locale)) return;
    this.locale = locale;
    writeStored(STORAGE_KEY, locale);
    this.#apply();
  }

  #apply() {
    setCoreLocale(this.locale);
    if (typeof document !== "undefined") {
      document.documentElement.lang = this.locale === "zh" ? "zh-CN" : "en";
    }
  }

  /** Translate key, falling back to English and then to the key itself. */
  t(key, params) {
    // Reading the rune makes callers reactive to language changes.
    void this.locale;
    return coreT(key, params);
  }

  isZh() {
    return this.locale === "zh";
  }
}

export const i18n = new I18n();

export function t(key, params) {
  return i18n.t(key, params);
}

export { getLocale };
