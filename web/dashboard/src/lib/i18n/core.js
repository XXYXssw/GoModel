// Rune-free i18n core shared by plain logic modules (which node:test loads
// directly, so they cannot use Svelte runes). The rune store in
// i18n.svelte.js owns the reactive locale for components and delegates here;
// logic modules call t() from this file, defaulting to English in tests.
import { dictionaries } from "./dictionaries/index.js";

let currentLocale = "en";

export function setLocale(locale) {
  if (Object.prototype.hasOwnProperty.call(dictionaries, locale)) {
    currentLocale = locale;
  }
}

export function getLocale() {
  return currentLocale;
}

export function t(key, params) {
  const dict = dictionaries[currentLocale] || dictionaries.en;
  let text = dict[key] ?? dictionaries.en[key] ?? key;
  if (params) {
    for (const [name, value] of Object.entries(params)) {
      text = String(text).replaceAll("{" + name + "}", String(value));
    }
  }
  return text;
}
