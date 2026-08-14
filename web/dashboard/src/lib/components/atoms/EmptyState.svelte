<script>
  // Empty-table / empty-list placeholder.
  import Icon from "./Icon.svelte";
  import { Inbox } from "lucide";
  import { t } from "$lib/i18n/i18n.svelte.js";

  let { icon = Inbox, title = t("common.noData"), hint = "", children } = $props();
</script>

<div class="empty-state">
  <Icon {icon} class="empty-state-icon" />
  <p class="empty-state-title">{title}</p>
  {#if hint}
    <p class="empty-state-hint">{hint}</p>
  {/if}
  {@render children?.()}
</div>
