<script>
  // Budgets page (list, editor, per-budget reset, delete flows).
  import LoadingState from "$lib/components/molecules/LoadingState.svelte";
  import AuthBanner from "$lib/components/organisms/AuthBanner.svelte";
  import { router } from "$lib/stores/router.svelte.js";
  import { auth } from "$lib/stores/auth.svelte.js";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import FilterInput from "$lib/components/molecules/FilterInput.svelte";
  import InlineHelpSection from "$lib/components/molecules/InlineHelpSection.svelte";
  import BudgetList from "./BudgetList.svelte";
  import BudgetEditor from "./BudgetEditor.svelte";
  import { budgetsStore as store } from "./budgets.svelte.js";
  import { Plus } from "lucide";
  import { t } from "$lib/i18n/i18n.svelte.js";

  const PAGE = "budgets";

  // Re-fetch when the page becomes active or the API key changes.
  $effect(() => {
    void auth.refreshTick;
    if (router.page === PAGE) store.fetchBudgetsPage();
  });

  const filtered = $derived(store.filteredBudgets());
</script>

<div>
  <div class="page-header">
    <div>
      <InlineHelpSection copyId="budgets-help-copy" label={t("budgets.helpLabel")}>
        {#snippet title()}<h2>{t("budgets.title")}</h2>{/snippet}
        {#snippet help()}
          Budgets are evaluated from tracked usage cost records for each user
          path subtree. Enforcement runs only when Budget is enabled for the
          active workflow.
        {/snippet}
      </InlineHelpSection>
    </div>
    <div class="page-header-controls">
      {#if store.managementEnabled() && store.budgetsAvailable && !auth.authError}
        <button
          type="button"
          class="btn btn-primary btn-with-icon"
          disabled={store.formSubmitting}
          onclick={() => store.openForm()}
        >
          <Icon icon={Plus} class="form-action-icon" />
          <span>{t("budgets.create")}</span>
        </button>
      {/if}
    </div>
  </div>

  <AuthBanner />

  {#if (!store.managementEnabled() || !store.budgetsAvailable) && !auth.authError}
    <div class="alert alert-warning">{t("budgets.unavailable")}</div>
  {/if}
  {#if store.error && !auth.authError}
    <p class="form-error" role="alert" aria-live="assertive">{store.error}</p>
  {/if}
  {#if store.loading && !auth.authError}
    <LoadingState label={t("budgets.loading")} />
  {/if}

  {#if (store.budgets.length > 0 || store.filter) && store.budgetsAvailable && !auth.authError && !store.formOpen}
    <div class="table-toolbar">
      <div class="table-toolbar-main">
        <FilterInput
          id="budget-filter"
          placeholder={t("budgets.filterPlaceholder")}
          label={t("budgets.filterLabel")}
          bind:value={store.filter}
        />
      </div>
      <div class="table-toolbar-actions budget-sort-control">
        <label for="budget-sort-by">{t("budgets.sortBy")}</label>
        <select
          id="budget-sort-by"
          class="usage-log-select budget-sort-select"
          aria-label={t("budgets.sortByLabel")}
          bind:value={store.sortBy}
        >
          <option value="subject">{t("budgets.scopeSubject")}</option>
          <option value="period">{t("budgets.period")}</option>
        </select>
      </div>
    </div>
  {/if}

  <BudgetEditor />

  {#if filtered.length > 0 && store.budgetsAvailable && !auth.authError}
    <BudgetList budgets={filtered} />
  {/if}

  {#if store.budgets.length === 0 && !store.filter && !store.loading && !auth.authError && !store.error && store.budgetsAvailable && store.managementEnabled()}
    <p class="empty-state">{t("budgets.empty")}</p>
  {/if}
  {#if store.budgets.length > 0 && filtered.length === 0 && store.filter && !store.loading && !auth.authError && !store.error && store.budgetsAvailable && store.managementEnabled()}
    <p class="empty-state">{t("budgets.emptyFilter")}</p>
  {/if}
</div>

<style>
  .budget-sort-control {
    align-items: center;
    gap: 8px;
  }

  .budget-sort-control :global(label) {
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
  }

  .budget-sort-select {
    background-color: var(--bg-surface);
    min-width: 132px;
  }

  .budget-sort-select:hover {
    background-color: var(--bg-surface-hover);
  }
</style>
