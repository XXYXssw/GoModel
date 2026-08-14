<script>
  // EN/中 language switcher for the sidebar footer and Settings page.
  import { i18n } from "$lib/i18n/i18n.svelte.js";
  import { t } from "$lib/i18n/i18n.svelte.js";

  const locales = [
    { value: "en", label: "EN" },
    { value: "zh", label: "中" },
  ];
</script>

<div class="language-toggle" role="group" aria-label={t("common.language")}>
  {#each locales as locale (locale.value)}
    <button
      type="button"
      class="language-btn"
      class:active={i18n.locale === locale.value}
      aria-pressed={i18n.locale === locale.value}
      title={locale.value === "en" ? t("common.english") : t("common.chinese")}
      onclick={() => i18n.setLocale(locale.value)}
    >
      {locale.label}
    </button>
  {/each}
</div>

<style>
  .language-toggle {
    display: inline-flex;
    align-items: center;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 2px;
    margin-bottom: 10px;
  }

  .language-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    min-width: 28px;
    height: 24px;
    padding: 0 6px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s;
  }

  .language-btn:hover {
    color: var(--text);
  }

  .language-btn.active {
    background: var(--accent);
    color: #fff;
  }

  .language-btn:focus-visible {
    outline: 2px solid color-mix(in srgb, var(--accent) 36%, transparent);
    outline-offset: 2px;
  }
</style>
