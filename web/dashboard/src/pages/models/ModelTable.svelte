<script>
  // Grouped models table for the active category — one component that
  // switches its columns per category. Provider groups are collapsible; the
  // collapsed set persists in localStorage and is ignored while a filter is
  // active so search results always stay visible.
  import Icon from "$lib/components/atoms/Icon.svelte";
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import { modelsStore } from "$lib/stores/models.svelte.js";
  import { readStored, writeStored } from "$lib/utils/storage.js";
  import { t } from "$lib/i18n/i18n.svelte.js";
  import { virtualModels } from "./virtualModels.svelte.js";
  import { pricingOverrides } from "./pricingOverrides.svelte.js";
  import { rateLimits } from "$pages/rate-limits/rateLimits.svelte.js";
  import {
    hasAccessOverride,
    modelOverrideEditButtonClass,
    modelOverrideEditButtonLabel,
  } from "./virtualModelsLogic.js";
  import AccessToggle from "./AccessToggle.svelte";
  import ModelGlobalActions from "./ModelGlobalActions.svelte";
  import ModelRow from "./ModelRow.svelte";
  import { categoryColumns, categoryColspan } from "./categoryColumns.js";
  import { ChevronDown, ChevronRight, CircleDollarSign, Gauge, Pencil } from "lucide";

  const COLLAPSE_KEY = "gomodel.collapsedModelGroups";

  const category = $derived(modelsStore.activeCategory || "all");
  const columns = $derived(categoryColumns(category));
  const groupColspan = $derived(categoryColspan(category));

  const groups = $derived(virtualModels.filteredDisplayModelGroups);

  let collapsedGroups = $state(loadCollapsedGroups());

  function loadCollapsedGroups() {
    const raw = readStored(COLLAPSE_KEY);
    if (!raw) return new Set();
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? new Set(parsed) : new Set();
    } catch {
      return new Set();
    }
  }

  function persistCollapsed() {
    writeStored(COLLAPSE_KEY, JSON.stringify([...collapsedGroups]));
  }

  function isCollapsed(key) {
    return !modelsStore.filter && collapsedGroups.has(key);
  }

  function toggleGroup(key) {
    if (collapsedGroups.has(key)) {
      collapsedGroups.delete(key);
    } else {
      collapsedGroups.add(key);
    }
    persistCollapsed();
  }

  const allCollapsed = $derived(
    groups.length > 0 && groups.every((group) => isCollapsed(group.key)),
  );

  function toggleAll() {
    const shouldCollapse = !allCollapsed;
    collapsedGroups = new Set();
    if (shouldCollapse) {
      for (const group of groups) collapsedGroups.add(group.key);
    }
    persistCollapsed();
  }
</script>

{#if groups.length > 0 && !modelsStore.filter}
  <div class="model-table-toolbar">
    <button
      type="button"
      class="btn btn-sm"
      onclick={toggleAll}
    >
      {allCollapsed ? t("models.expandAll") : t("models.collapseAll")}
    </button>
  </div>
{/if}

<div class="table-wrapper">
  <table class="data-table">
    <thead>
      <tr>
        <th>{t("models.column.model")}</th>
        {#each columns as col, i (i)}
          <th class={col.class}>
            {#each col.headerLines as line, li (li)}
              {#if li > 0}<br />{/if}{line}
            {/each}
          </th>
        {/each}
        <th class="model-actions-header"><ModelGlobalActions /></th>
      </tr>
    </thead>
    {#each groups as group (group.key)}
      <tbody>
        <tr class="provider-group-row">
          <td colspan={groupColspan}>
            <div class="provider-group-header">
              <button
                type="button"
                class="provider-group-toggle"
                aria-expanded={!isCollapsed(group.key)}
                onclick={() => toggleGroup(group.key)}
              >
                <Icon
                  icon={isCollapsed(group.key) ? ChevronRight : ChevronDown}
                  class="provider-group-chevron"
                />
                <span class="provider-group-meta">
                  <span class="provider-group-title">
                    <span class="mono font-size-md">{group.display_name}</span>
                    {#if group.type_label}
                      <span class="provider-group-type">{"(" + group.type_label + ")"}</span>
                    {/if}
                    {#if group.item_count_label}
                      <span class="provider-group-count">{group.item_count_label}</span>
                    {/if}
                  </span>
                  {#if group.access_summary}
                    <span class="provider-group-summary">{group.access_summary}</span>
                  {/if}
                </span>
              </button>
              <div class="alias-actions-cell model-list-actions">
                {#if group.access.selector}
                  <AccessToggle row={group} />
                {/if}
                {#if pricingOverrides.modelPricingOverridesAvailable && group.provider_name}
                  <TableActionButton
                    label={pricingOverrides.modelPricingButtonLabel(t("models.providerPricingFor", { name: group.display_name }), pricingOverrides.hasProviderPricingOverride(group))}
                    class="table-icon-btn {pricingOverrides.modelPricingButtonClass(pricingOverrides.hasProviderPricingOverride(group))}"
                    onclick={() => pricingOverrides.openProviderPricingOverrideEdit(group)}
                  >
                    <Icon icon={CircleDollarSign} class="table-icon-svg" />
                  </TableActionButton>
                {/if}
                {#if rateLimits.rateLimitsEnabled() && group.provider_name}
                  <TableActionButton
                    label={rateLimits.rateLimitGaugeTitle(t("models.providerRateLimitFor", { name: group.display_name }), rateLimits.rateLimitGaugeClassForProvider(group))}
                    class="table-icon-btn {rateLimits.rateLimitGaugeClassForProvider(group)}"
                    onclick={() => rateLimits.openRateLimitInspectorForProvider(group)}
                  >
                    <Icon icon={Gauge} class="table-icon-svg" />
                  </TableActionButton>
                {/if}
                {#if virtualModels.virtualModelsAvailable && group.access.selector}
                  <TableActionButton
                    label={modelOverrideEditButtonLabel(t("models.providerAccessFor", { name: group.display_name }), hasAccessOverride(group.access))}
                    class="table-icon-btn {modelOverrideEditButtonClass(hasAccessOverride(group.access))}"
                    onclick={() => virtualModels.openProviderOverrideEdit(group)}
                  >
                    <Icon icon={Pencil} class="table-icon-svg" />
                  </TableActionButton>
                {/if}
              </div>
            </div>
          </td>
        </tr>
        {#if !isCollapsed(group.key)}
          {#each group.rows as row (row.key)}
            <ModelRow {row} {columns} />
          {/each}
        {/if}
      </tbody>
    {/each}
  </table>
</div>

<style>
  .model-table-toolbar {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 8px;
  }

  .provider-group-row :global(td) {
    background: color-mix(in srgb, var(--accent) 6%, var(--bg));
    padding-top: 6px;
    padding-bottom: 6px;
  }

  .provider-group-row:hover :global(td) {
    background: color-mix(in srgb, var(--accent) 8%, var(--bg));
  }

  .provider-group-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .provider-group-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    background: transparent;
    border: none;
    padding: 6px 4px;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
    border-radius: 6px;
  }

  .provider-group-toggle:hover {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
  }

  .provider-group-toggle:focus-visible {
    outline: 2px solid color-mix(in srgb, var(--accent) 36%, transparent);
    outline-offset: 2px;
  }

  .provider-group-chevron {
    width: 16px;
    height: 16px;
    flex: 0 0 16px;
    color: var(--text-muted);
  }

  .provider-group-meta {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .provider-group-title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .provider-group-type, .provider-group-count, .provider-group-summary {
    color: var(--text-muted);
  }

  .provider-group-type, .provider-group-count {
    font-size: 12px;
  }

  .provider-group-summary {
    font-size: 12px;
  }

  @media (max-width: 768px) {
    .provider-group-header {
        flex-direction: column;
        align-items: flex-start;
      }
  }
</style>
