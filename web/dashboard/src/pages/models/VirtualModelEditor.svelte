<script>
  // Unified virtual-model editor (redirect/alias, load balancer, or access
  // policy) on the shared EditorDialog shell.
  import EditorDialog from "$lib/components/organisms/EditorDialog.svelte";
  import EnabledToggle from "$lib/components/atoms/EnabledToggle.svelte";
  import FormField from "$lib/components/molecules/FormField.svelte";
  import InlineHelpSection from "$lib/components/molecules/InlineHelpSection.svelte";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import { modelsStore } from "$lib/stores/models.svelte.js";
  import { runtimeConfig } from "$lib/stores/runtimeConfig.svelte.js";
  import { virtualModels } from "./virtualModels.svelte.js";
  import { qualifiedModelName } from "./virtualModelsLogic.js";
  import VmTargetRow from "./VmTargetRow.svelte";
  import { Plus, Save } from "lucide";
  import { t } from "$lib/i18n/i18n.svelte.js";

  const vm = virtualModels;

  // The strategy dropdown is server-driven (VIRTUAL_MODEL_STRATEGIES); make
  // sure the runtime config is loaded by the time the editor shows it.
  $effect(() => {
    if (vm.vmFormOpen) {
      runtimeConfig.ensureLoaded();
    }
  });
</script>

<EditorDialog
  open={vm.vmFormOpen}
  ariaLabel={t("vm.editorAria")}
  error={vm.vmFormError}
  submitting={vm.vmSubmitting}
  submitDisabled={vm.vmDeleting || vm.vmFormManaged}
  submitLabel={vm.vmFormMode === "edit" ? t("common.save") : t("common.create")}
  submitIcon={vm.vmFormMode !== "edit" ? Plus : Save}
  onclose={() => vm.closeVirtualModelForm()}
  onsubmit={() => vm.submitVirtualModelForm()}
>
  {#snippet header()}
    <InlineHelpSection
      copyId="virtual-model-help-copy"
      label={t("vm.helpLabel")}
      bind:open={vm.vmFormHelpOpen}
    >
      {#snippet title()}
        <h3>{vm.vmFormDisplayName || vm.vmForm.source || t("vm.virtualModel")}</h3>
      {/snippet}
      {#snippet help()}
        {@html t("vm.helpText")}{#if runtimeConfig.virtualModelStrategies().includes("adaptive")}{@html t("vm.helpTextAdaptive")}{/if}
      {/snippet}
    </InlineHelpSection>
  {/snippet}

  {#if vm.vmFormManaged}
    <p class="form-hint" role="status">
      {@html t("vm.managedHint")}
    </p>
  {/if}

  <FormField id="virtual-model-source" label={t("vm.source")}>
    <input
      id="virtual-model-source"
      type="text"
      class="mono"
      placeholder="smart"
      bind:value={vm.vmForm.source}
      disabled={vm.vmFormSourceLocked || vm.vmFormManaged}
      data-modal-autofocus
    />
  </FormField>

  <datalist id="virtual-model-target-options">
    {#each modelsStore.models as m (qualifiedModelName(m))}
      <option value={qualifiedModelName(m)}>{qualifiedModelName(m)}</option>
    {/each}
  </datalist>

  <!-- Every target renders the same; two or more turn the redirect into a load balancer. -->
  <div class="form-field">
    <span class="form-field-label">{t("vm.targets")}</span>
    <VmTargetRow
      id="virtual-model-target"
      bind:model={vm.vmForm.target_model}
      bind:weight={vm.vmForm.target_weight}
      showRemove={vm.vmFormHasPrimaryTarget()}
      onremove={() => vm.removePrimaryTarget()}
    />
    {#each vm.vmForm.targets as target, index (index)}
      <VmTargetRow
        placeholder="groq/llama"
        bind:model={target.model}
        bind:weight={target.weight}
        onremove={() => vm.removeVmTarget(index)}
      />
    {/each}
    <div>
      <button
        type="button"
        class="btn btn-with-icon"
        disabled={vm.vmFormManaged}
        onclick={() => vm.addVmTarget()}
      >
        <Icon icon={Plus} class="form-action-icon" />
        <span>{t("vm.addTarget")}</span>
      </button>
    </div>
  </div>

  {#if vm.vmFormShowStrategy()}
    <FormField id="virtual-model-strategy" label={t("vm.strategy")}>
      <select
        id="virtual-model-strategy"
        class="form-select"
        bind:value={vm.vmForm.strategy}
        disabled={vm.vmFormManaged}
      >
        {#each vm.vmStrategyOptions() as option (option.value)}
          <option value={option.value}>{t(option.label)}</option>
        {/each}
      </select>
    </FormField>
    <div class="form-field">
      <label class="vm-session-affinity-checkbox">
        <input
          type="checkbox"
          bind:checked={vm.vmForm.session_affinity}
          disabled={vm.vmFormManaged}
        />
        <span>{t("vm.sessionKeeping")}</span>
      </label>
    </div>
  {/if}

  <div class="form-field">
    <InlineHelpSection
      copyId="virtual-model-user-paths-help"
      label={t("vm.userPathsHelpLabel")}
      bind:open={vm.vmFormUserPathsHelpOpen}
    >
      {#snippet title()}
        <label class="form-field-label" for="virtual-model-user-paths">{t("vm.userPaths")}</label>
      {/snippet}
      {#snippet help()}
        {@html t("vm.userPathsHelp")}
      {/snippet}
    </InlineHelpSection>
    <textarea
      id="virtual-model-user-paths"
      rows="4"
      class="mono"
      placeholder={"/\n/team/alpha\n/non-existing"}
      bind:value={vm.vmForm.user_paths}
      disabled={vm.vmFormManaged}
    ></textarea>
  </div>

  <FormField id="virtual-model-description" label={t("common.description")}>
    <textarea
      id="virtual-model-description"
      rows="3"
      placeholder={t("vm.optionalDescription")}
      bind:value={vm.vmForm.description}
      disabled={vm.vmFormManaged}
    ></textarea>
  </FormField>

  {#if vm.vmFormSupportsSlowdown()}
    <FormField id="virtual-model-slowdown" label={t("vm.slowdown")}>
      <input
        id="virtual-model-slowdown"
        type="number"
        min="0"
        max="10"
        step="0.1"
        placeholder={t("vm.off")}
        bind:value={vm.vmForm.slowdown}
        disabled={vm.vmFormManaged}
      />
      <span class="form-hint">{@html t("vm.slowdownHint")}</span>
    </FormField>
  {/if}

  {#if vm.vmFormThinkingVisible()}
    <div class="form-field">
      <span class="form-field-label">{t("vm.thinkingOverride")}</span>
      <span class="form-hint">{@html t("vm.thinkingOverrideHint")}</span>
      <select
        id="thinking-override-mode"
        class="form-select"
        bind:value={vm.vmFormThinking.mode}
        disabled={vm.vmFormManaged}
      >
        <option value="native">{t("vm.thinkingModeNative")}</option>
        <option value="supplement">{t("vm.thinkingModeSupplement")}</option>
        <option value="override">{t("vm.thinkingModeOverride")}</option>
      </select>
    </div>

    {#if vm.vmFormThinking.mode !== "native"}
      <div class="form-field">
        <label class="vm-session-affinity-checkbox">
          <input
            type="checkbox"
            bind:checked={vm.vmFormThinking.thinking}
            disabled={vm.vmFormManaged}
          />
          <span>{t("vm.thinkingOn")}</span>
        </label>
      </div>
      {#if vm.vmFormThinking.thinking}
        <FormField id="thinking-override-effort" label={t("vm.thinkingLevel")}>
          <select
            id="thinking-override-effort"
            class="form-select"
            bind:value={vm.vmFormThinking.effort}
            disabled={vm.vmFormManaged}
          >
            <option value="low">{t("vm.effortLow")}</option>
            <option value="medium">{t("vm.effortMedium")}</option>
            <option value="high">{t("vm.effortHigh")}</option>
            <option value="xhigh">{t("vm.effortXHigh")}</option>
          </select>
        </FormField>
      {/if}
    {/if}
  {/if}

  <div class="vm-status-row">
    {#if vm.vmFormMode === "edit"}
      <span class="form-hint vm-status-summary">
        {t("vm.defaultEnabledSummary", { value: vm.vmFormDefaultEnabled ? t("common.yes") : t("common.no") }) +
          " · " +
          t("vm.effectiveNowSummary", { value: vm.vmFormEffectiveEnabled ? t("common.yes") : t("common.no") })}
      </span>
    {/if}
    <div class="vm-status-toggle">
      <EnabledToggle
        enabled={vm.vmForm.enabled}
        restricted={vm.vmFormToggleRestricted()}
        label={t("vm.virtualModel")}
        disabled={vm.vmFormManaged}
        text={vm.vmFormToggleLabel()}
        onclick={() => {
          if (!vm.vmFormManaged) {
            vm.vmForm.enabled = !vm.vmForm.enabled;
          }
        }}
      />
    </div>
  </div>

  {#snippet extraActions()}
    {#if vm.vmFormHasExisting && !vm.vmFormManaged}
      <button
        type="button"
        class="btn btn-danger-outline"
        disabled={vm.vmDeleting || vm.vmSubmitting}
        onclick={() => vm.deleteVirtualModel()}
      >
        {t("common.remove")}
      </button>
    {/if}
  {/snippet}
</EditorDialog>

<style>
  .vm-session-affinity-checkbox {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text);
    cursor: pointer;
    user-select: none;
  }

  .vm-session-affinity-checkbox input {
    accent-color: var(--accent);
    cursor: pointer;
  }
</style>
