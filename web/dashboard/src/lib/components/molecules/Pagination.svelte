<script>
  // Offset/limit pagination bar.
  import { t } from "$lib/i18n/i18n.svelte.js";
  let { total = 0, offset = 0, limit = 25, onprev, onnext } = $props();
</script>

{#if total > 0}
  <div class="pagination">
    <span class="pagination-info">
      {t("pagination.showing", { from: offset + 1, to: Math.min(offset + limit, total), total })}
    </span>
    <div class="pagination-buttons">
      <button
        type="button"
        class="btn"
        disabled={offset === 0}
        onclick={() => onprev?.()}>{t("common.prev")}</button
      >
      <button
        type="button"
        class="btn"
        disabled={offset + limit >= total}
        onclick={() => onnext?.()}>{t("common.next")}</button
      >
    </div>
  </div>
{/if}

<style>
  .pagination-info {
    font-size: 13px;
    color: var(--text-muted);
  }

  .pagination-buttons {
    display: flex;
    gap: 8px;
  }
</style>
