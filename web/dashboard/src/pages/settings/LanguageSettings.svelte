<script>
  // Language preference: EN/中 picker backed by the shared i18n store.
  import LanguageToggle from "$lib/components/organisms/LanguageToggle.svelte";
  import { t } from "$lib/i18n/i18n.svelte.js";
</script>

<div class="settings-section">
  <div class="settings-section-head">
    <h3>{t("settings.language")}</h3>
    <LanguageToggle />
  </div>
  <p class="settings-hint">{t("settings.languageDescription")}</p>
</div>

<style>
  .settings-section {
    padding: 16px 0;
    border-bottom: 1px solid var(--border);
  }

  .settings-section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .settings-section-head :global(.language-toggle) {
    margin-bottom: 0;
  }

  .settings-hint {
    margin: 6px 0 0;
    color: var(--text-muted);
    font-size: 13px;
  }
</style>
