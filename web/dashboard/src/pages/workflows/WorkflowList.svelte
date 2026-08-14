<script>
  // Active workflow card grid with loading/empty states.
  import Spinner from "$lib/components/atoms/Spinner.svelte";
  import { auth } from "$lib/stores/auth.svelte.js";
  import { workflowsStore as wf } from "./workflows.svelte.js";
  import WorkflowCard from "./WorkflowCard.svelte";
  import { t } from "$lib/i18n/i18n.svelte.js";
</script>

<section class="workflows-list">
  {#if wf.loading && !auth.authError}
    <p class="empty-state workflow-list-loading">
      <Spinner size={16} label={t("workflows.loading")} />
      {t("workflows.loading")}...
    </p>
  {/if}

  {#if wf.filteredWorkflows.length > 0}
    <div class="workflow-card-grid">
      {#each wf.filteredWorkflows as workflow (workflow.id)}
        <WorkflowCard {workflow} />
      {/each}
    </div>
  {/if}

  {#if wf.workflows.length === 0 && !wf.loading && !auth.authError && wf.available}
    <p class="empty-state">{t("workflows.empty")}</p>
  {/if}
  {#if wf.workflows.length > 0 && wf.filteredWorkflows.length === 0 && !wf.loading}
    <p class="empty-state">{t("workflows.emptyFilter")}</p>
  {/if}
</section>

<style>
  /* Loading affordance. */
  .workflow-list-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }
.workflows-list {
  min-width: 0;
}

.workflow-card-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
}
</style>
