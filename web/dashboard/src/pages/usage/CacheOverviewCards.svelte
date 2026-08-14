<script>
  // Cache overview stat cards (rendered inside the page's .cards grid).
  // Values come from the shared cacheOverview, which the usage page fetches
  // with its facet filters applied.
  import { usageData } from "$lib/stores/usageData.svelte.js";
  import { t } from "$lib/i18n/i18n.svelte.js";
  import { formatCost, formatNumber } from "$lib/utils/format.js";
</script>

{#if usageData.cacheAnalyticsEnabled()}
  <div class="card">
    <div class="card-label">{t("usage.cacheSaved")}</div>
    <div class="card-value">
      {formatCost(usageData.cacheOverview.summary.total_saved_cost)}
    </div>
  </div>
  <div class="card">
    <div class="card-label">{t("usage.cacheHits")}</div>
    <div class="card-value">
      {formatNumber(usageData.cacheOverview.summary.total_hits)}
    </div>
  </div>
{/if}
