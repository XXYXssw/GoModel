// Virtual models (redirects/aliases, load balancers, access policies) state
// for the Models page.

import { errorMessage, getJSON, sendJSON } from "$lib/api/client.js";
import { t } from "$lib/i18n/i18n.svelte.js";
import { flash } from "$lib/stores/flash.svelte.js";
import { modelsStore } from "$lib/stores/models.svelte.js";
import { runtimeConfig } from "$lib/stores/runtimeConfig.svelte.js";
import {
  GLOBAL_OVERRIDE_SELECTOR,
  aliasFormTargets,
  buildAliasTogglePayload,
  buildDisplayModels,
  buildGlobalScopeRow,
  buildModelTogglePayload,
  buildVirtualModelSavePayload,
  computeRenderStep,
  defaultThinkingForm,
  defaultVirtualModelForm,
  filterDisplayModels,
  findModelOverrideView,
  groupDisplayModels,
  initialRenderStep,
  modelAccessStateClass,
  modelKeys,
  modelOverridesDefaultEnabled,
  normalizeUserPaths,
  normalizedAliasName,
  qualifiedModelName,
  rowAccessSelector,
  rowIsManaged,
  splitVirtualModelViews,
  strategyOptions,
  thinkingFormVisibleForSource,
  vmFormHasPrimaryTarget,
  vmFormShowStrategy,
  vmFormShowWeights,
  vmFormSupportsSlowdown,
  modelAccessUserPathsRestrict,
  removePrimaryTarget as removePrimaryTargetPure,
} from "./virtualModelsLogic.js";

class VirtualModelsStore {
  virtualModelsAvailable = $state(true);
  aliases = $state([]);
  modelOverrideViews = $state([]);
  modelRenderLimit = $state(0);
  modelRenderBatchSize = 75;
  modelsRendering = $state(false);
  #renderGeneration = 0;
  aliasLoading = $state(false);
  // Load failures only; mutation feedback goes through the flash store.
  aliasError = $state("");
  rowTogglingKey = $state("");
  rowDeletingKey = $state("");

  // Unified editor (redirects/aliases and access policies share it).
  vmFormOpen = $state(false);
  vmFormHelpOpen = $state(false);
  vmFormUserPathsHelpOpen = $state(false);
  vmFormMode = $state("create");
  vmFormError = $state("");
  vmSubmitting = $state(false);
  vmDeleting = $state(false);
  vmFormHasExisting = $state(false);
  vmFormDefaultEnabled = $state(true);
  vmFormEffectiveEnabled = $state(true);
  vmFormDisplayName = $state("");
  vmFormSourceLocked = $state(false);
  vmFormOriginalSource = $state("");
  vmFormManaged = $state(false);
  vmForm = $state(defaultVirtualModelForm());
  // Per-model thinking override settings (dashboard "思考覆盖" section).
  thinkingOverrides = $state([]);
  vmFormThinking = $state(defaultThinkingForm());

  // ---- Display rows (derived from the shared model inventory) ----

  displayModels = $derived(
    buildDisplayModels({
      models: modelsStore.models,
      aliases: this.aliases,
      virtualModelsAvailable: this.virtualModelsAvailable,
      activeCategory: modelsStore.activeCategory,
    }),
  );

  displayModelGroups = $derived(
    groupDisplayModels(this.displayModels, modelsStore.models, this.modelOverrideViews),
  );

  filteredDisplayModels = $derived(filterDisplayModels(this.displayModels, modelsStore.filter));

  filteredDisplayModelGroups = $derived.by(() => {
    const filtered = this.filteredDisplayModels;
    const limit = Math.max(0, Math.min(Number(this.modelRenderLimit || 0), filtered.length));
    if (!modelsStore.filter && limit >= this.displayModels.length) {
      return this.displayModelGroups;
    }
    return groupDisplayModels(filtered.slice(0, limit), modelsStore.models, this.modelOverrideViews);
  });

  // globalScopeRow exposes the global "/" scope as a toggle row, so the global
  // level reuses the same enable/restrict/disable switch as models, aliases,
  // and provider groups.
  globalScopeRow = $derived(buildGlobalScopeRow(modelsStore.models, this.modelOverrideViews));

  modelsBusy() {
    return Boolean(modelsStore.loading || this.modelsRendering);
  }

  modelLoadingText() {
    if (modelsStore.loading) {
      return this.displayModels.length > 0 ? t("models.refreshing") : t("models.loading");
    }
    const total = this.filteredDisplayModels.length;
    const visible = Math.min(Number(this.modelRenderLimit || 0), total);
    return t("models.rendering", { visible, total });
  }

  // ---- Incremental render batching (loader paints between batches) ----
  //
  // The Models page owns the lifecycle: an $effect calls
  // restartModelRendering(total) whenever the visible row set changes and
  // stopModelRendering() on teardown. Batches grow the window between
  // animation frames; the generation counter cancels a stale run when a
  // newer restart (or a stop) supersedes it.

  restartModelRendering(total) {
    const generation = ++this.#renderGeneration;
    const step = initialRenderStep(this.modelRenderBatchSize, total);
    this.modelRenderLimit = step.limit;
    this.modelsRendering = step.rendering;
    if (step.rendering) {
      this.#scheduleRenderBatch(generation);
    }
  }

  stopModelRendering() {
    this.#renderGeneration++;
    this.modelsRendering = false;
  }

  #scheduleRenderBatch(generation) {
    const run = () => {
      if (generation !== this.#renderGeneration) {
        return;
      }
      const step = computeRenderStep(
        this.modelRenderLimit,
        this.modelRenderBatchSize,
        this.filteredDisplayModels.length,
      );
      this.modelRenderLimit = step.limit;
      this.modelsRendering = step.rendering;
      if (step.rendering) {
        this.#scheduleRenderBatch(generation);
      }
    };
    if (typeof requestAnimationFrame === "function") {
      requestAnimationFrame(() => setTimeout(run, 0));
    } else {
      setTimeout(run, 0);
    }
  }

  // ---- Fetch ----

  async fetchVirtualModels() {
    this.aliasLoading = true;
    this.aliasError = "";
    try {
      const result = await getJSON("/admin/virtual-models", { label: "virtual models" });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        this.aliases = [];
        this.modelOverrideViews = [];
        return;
      }
      if (result.stale) {
        return;
      }
      this.virtualModelsAvailable = true;
      if (!result.ok) {
        this.aliases = [];
        this.modelOverrideViews = [];
        return;
      }
      const { aliases, policies } = splitVirtualModelViews(result.data);
      this.aliases = aliases;
      this.modelOverrideViews = policies;
    } catch (e) {
      console.error("Failed to fetch virtual models:", e);
      this.aliases = [];
      this.modelOverrideViews = [];
      this.aliasError = "Unable to load virtual models.";
    } finally {
      this.aliasLoading = false;
    }
  }

  // ---- Lookups ----

  qualifiedModelName(model) {
    return qualifiedModelName(model);
  }

  findModelOverrideView(selector) {
    return findModelOverrideView(this.modelOverrideViews, selector);
  }

  hasGlobalModelOverride() {
    return Boolean(this.findModelOverrideView(GLOBAL_OVERRIDE_SELECTOR));
  }

  findExistingAliasByName(name) {
    const normalizedName = normalizedAliasName(name);
    if (!normalizedName) {
      return null;
    }
    for (const alias of this.aliases) {
      if (normalizedAliasName(alias && alias.name) === normalizedName) {
        return alias;
      }
    }
    return null;
  }

  findConcreteModelByName(name) {
    const normalizedName = normalizedAliasName(name);
    if (!normalizedName) {
      return null;
    }
    for (const model of modelsStore.models) {
      if (modelKeys(model).has(normalizedName)) {
        return model;
      }
    }
    return null;
  }

  // ---- Row enable/disable toggle (real models, aliases, groups, global) ----

  rowToggleEnabled(row) {
    if (!row) {
      return false;
    }
    if (row.is_alias) {
      return row.alias && row.alias.enabled !== false;
    }
    return Boolean(row.access && row.access.effective_enabled !== false);
  }

  rowToggleLabel(row) {
    if (this.rowTogglingKey && this.rowTogglingKey === row.key) {
      return t("vm.updating");
    }
    if (this.rowToggleRestricted(row)) {
      return t("vm.restricted");
    }
    return this.rowToggleEnabled(row) ? t("common.enabled") : t("common.disabled");
  }

  rowToggleRestricted(row) {
    return (
      Boolean(row) &&
      !row.is_alias &&
      modelAccessStateClass(row.access, this.virtualModelsAvailable) === "is-restricted"
    );
  }

  rowToggleAriaLabel(row) {
    if (!row) {
      return "";
    }
    const action = this.rowToggleEnabled(row) ? t("common.disable") + " " : t("common.enable") + " ";
    let subject;
    if (row.is_alias) {
      subject = t("vm.alias") + " " + String((row.alias && row.alias.name) || "");
    } else {
      subject = String(row.display_name || (row.access && row.access.selector) || t("common.model"));
    }
    return action + subject.trim();
  }

  async toggleRowEnabled(row) {
    if (!this.virtualModelsAvailable) {
      return;
    }
    if (!row || this.rowTogglingKey === row.key) {
      return;
    }
    if (rowIsManaged(row)) {
      flash.success(t("vm.managedReadonly"));
      return;
    }
    if (row.is_alias) {
      await this.toggleAliasRow(row);
      return;
    }
    await this.toggleModelRow(row);
  }

  async toggleAliasRow(row) {
    const alias = row.alias;
    if (!alias || !alias.name) {
      return;
    }

    this.rowTogglingKey = row.key;

    const payload = buildAliasTogglePayload(alias);
    try {
      const result = await sendJSON("/admin/virtual-models", "PUT", payload, {
        label: "alias state",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        flash.error(t("models.virtualModelsUnavailable"));
        return;
      }
      if (result.stale) {
        return;
      }
      if (!result.ok) {
        flash.error(
          result.status === 401
            ? t("auth.required")
            : errorMessage(result, t("vm.updateAliasFailed")),
        );
        return;
      }

      flash.success(payload.enabled ? t("vm.aliasEnabled") : t("vm.aliasDisabled"));
      void this.fetchVirtualModels();
    } catch (e) {
      console.error("Failed to toggle alias state:", e);
      flash.error(t("vm.updateAliasFailed"));
    } finally {
      this.rowTogglingKey = "";
    }
  }

  async toggleModelRow(row) {
    const selector = rowAccessSelector(row);
    if (!selector) {
      return;
    }
    const existingPolicy = this.findModelOverrideView(selector);
    const { method, payload, desired } = buildModelTogglePayload(
      selector,
      existingPolicy,
      row.access || {},
    );

    this.rowTogglingKey = row.key;

    try {
      const result = await sendJSON("/admin/virtual-models", method, payload, {
        label: "model access",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        flash.error(t("models.virtualModelsUnavailable"));
        return;
      }
      if (!(method === "DELETE" && result.status === 404)) {
        if (result.stale) {
          return;
        }
        if (!result.ok) {
          flash.error(
            result.status === 401
              ? t("auth.required")
              : errorMessage(result, t("vm.updateModelAccessFailed")),
          );
          return;
        }
      }

      flash.success(desired ? t("vm.modelEnabled") : t("vm.modelDisabled"));
      void Promise.all([modelsStore.fetchModels(), this.fetchVirtualModels()]);
    } catch (e) {
      console.error("Failed to toggle model access:", e);
      flash.error(t("vm.updateModelAccessFailed"));
    } finally {
      this.rowTogglingKey = "";
    }
  }

  // ---- Row deletes ----

  async removeAliasRow(row) {
    if (!(row && row.is_alias && row.alias && row.alias.name && !row.alias.managed)) {
      return;
    }
    if (this.rowDeletingKey) {
      return;
    }
    const source = String(row.alias.name || "").trim();
    if (!source) {
      return;
    }
    await this.mutateVirtualModelRow({
      rowKey: row.key,
      confirmMessage: t("vm.removeAliasConfirm", { source }),
      method: "DELETE",
      payload: { source },
      operation: t("vm.operation"),
      failureMessage: t("vm.removeFailed"),
      notice: t("vm.removed"),
      ignoreNotFound: true,
    });
  }

  async removeRedirectRow(row) {
    const alias = row && row.masking_alias;
    if (!(row && !row.is_alias && alias && alias.name && !alias.managed)) {
      return;
    }
    if (this.rowDeletingKey) {
      return;
    }
    const source = String(alias.name || "").trim();
    if (!source) {
      return;
    }
    await this.mutateVirtualModelRow({
      rowKey: row.key,
      confirmMessage: t("vm.removeRedirectConfirm", { source }),
      method: "PUT",
      payload: {
        source,
        user_paths: Array.isArray(alias.user_paths) ? alias.user_paths : [],
        description: String(alias.description || "").trim(),
        enabled: alias.enabled !== false,
        ...(alias.slowdown != null ? { slowdown: Number(alias.slowdown) } : {}),
      },
      operation: t("vm.operationRedirect"),
      failureMessage: t("vm.removeRedirectFailed"),
      notice: t("vm.removedRedirect"),
    });
  }

  async mutateVirtualModelRow(options) {
    if (this.rowDeletingKey) {
      return;
    }
    if (!window.confirm(options.confirmMessage)) {
      return;
    }

    this.rowDeletingKey = options.rowKey;

    try {
      const result = await sendJSON("/admin/virtual-models", options.method, options.payload, {
        label: options.operation,
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        flash.error(t("models.virtualModelsUnavailable"));
        return;
      }
      if (!(options.ignoreNotFound && result.status === 404)) {
        if (result.stale) {
          return;
        }
        if (!result.ok) {
          flash.error(
            result.status === 401
              ? "Authentication required."
              : errorMessage(result, options.failureMessage),
          );
          return;
        }
      }
      this.virtualModelsAvailable = true;

      flash.success(options.notice);
      void Promise.all([modelsStore.fetchModels(), this.fetchVirtualModels()]);
    } catch (e) {
      console.error(options.failureMessage, e);
      flash.error(options.failureMessage);
    } finally {
      this.rowDeletingKey = "";
    }
  }

  // ---- Unified virtual-model editor ----

  addVmTarget() {
    if (!Array.isArray(this.vmForm.targets)) {
      this.vmForm.targets = [];
    }
    this.vmForm.targets.push({ model: "", weight: 1 });
  }

  removeVmTarget(index) {
    if (!Array.isArray(this.vmForm.targets)) {
      return;
    }
    this.vmForm.targets.splice(index, 1);
  }

  removePrimaryTarget() {
    removePrimaryTargetPure(this.vmForm);
  }

  vmFormHasPrimaryTarget() {
    return vmFormHasPrimaryTarget(this.vmForm);
  }

  vmFormShowStrategy() {
    return vmFormShowStrategy(this.vmForm);
  }

  // vmStrategyOptions builds the strategy dropdown from the strategies this
  // deployment supports (server-driven), keeping the edited row's current
  // value selectable even when the deployment no longer offers it.
  vmStrategyOptions() {
    return strategyOptions(runtimeConfig.virtualModelStrategies(), this.vmForm.strategy);
  }

  vmFormShowWeights() {
    return vmFormShowWeights(this.vmForm);
  }

  vmFormSupportsSlowdown() {
    return vmFormSupportsSlowdown(this.vmForm);
  }

  // The edit-modal status switch reuses the same .alias-toggle component and
  // three states, derived from the form's own fields: it is Restricted when
  // enabled and scoped to non-global user paths.
  vmFormToggleRestricted() {
    return (
      Boolean(this.vmForm && this.vmForm.enabled) &&
      modelAccessUserPathsRestrict(normalizeUserPaths(this.vmForm.user_paths))
    );
  }

  vmFormToggleLabel() {
    if (!this.vmForm || !this.vmForm.enabled) {
      return t("common.disabled");
    }
    return this.vmFormToggleRestricted() ? t("vm.restricted") : t("common.enabled");
  }

  resetVirtualModelForm() {
    this.vmFormError = "";
    this.vmFormHelpOpen = false;
    this.vmFormUserPathsHelpOpen = false;
    this.vmSubmitting = false;
    this.vmDeleting = false;
    this.vmFormHasExisting = false;
    this.vmFormDefaultEnabled = true;
    this.vmFormEffectiveEnabled = true;
    this.vmFormDisplayName = "";
    this.vmFormSourceLocked = false;
    this.vmFormOriginalSource = "";
    this.vmFormManaged = false;
    this.vmForm = defaultVirtualModelForm();
    this.vmFormThinking = defaultThinkingForm();
  }

  // fetchThinkingOverrides loads the per-model thinking override list used by
  // the model settings editor. A 503 means the feature is unavailable; the
  // editor simply hides its thinking section.
  async fetchThinkingOverrides() {
    try {
      const result = await getJSON("/admin/model-thinking-overrides", { label: "thinking overrides" });
      if (result.status === 503) {
        this.thinkingOverrides = [];
        return;
      }
      if (result.stale) {
        return;
      }
      this.thinkingOverrides = result.ok ? result.data || [] : [];
    } catch (e) {
      console.error("Failed to fetch thinking overrides:", e);
      this.thinkingOverrides = [];
    }
  }

  thinkingOverrideFor(selector) {
    if (!selector || !Array.isArray(this.thinkingOverrides)) {
      return null;
    }
    const trimmed = String(selector).trim();
    return (
      this.thinkingOverrides.find((o) => o && o.selector === trimmed) ||
      this.thinkingOverrides.find((o) => o && o.model === trimmed) ||
      null
    );
  }

  vmFormThinkingVisible() {
    return (
      !this.vmFormManaged &&
      thinkingFormVisibleForSource(this.vmForm.source || this.vmFormOriginalSource)
    );
  }

  loadThinkingIntoForm(selector) {
    const existing = this.thinkingOverrideFor(selector);
    this.vmFormThinking = existing
      ? {
          mode: existing.mode || "native",
          thinking: existing.thinking !== false,
          effort: existing.effort || "medium",
        }
      : defaultThinkingForm();
  }

  closeVirtualModelForm() {
    this.vmFormOpen = false;
    this.resetVirtualModelForm();
  }

  // openVirtualModelCreate opens the editor in create mode with an editable
  // Source. Optionally prefills the Target model from a model row.
  openVirtualModelCreate(model) {
    this.resetVirtualModelForm();
    this.vmFormOpen = true;
    this.vmFormMode = "create";
    this.vmFormSourceLocked = false;
    this.vmFormDisplayName = t("models.newVirtualModel");
    if (model && model.model && model.model.id) {
      this.vmForm.target_model = qualifiedModelName(model);
    }
  }

  // openVirtualModelEditAlias edits an existing redirect/alias. Source is
  // editable: changing it renames the virtual model (the original source
  // travels with the save as old_source so the backend moves the row).
  openVirtualModelEditAlias(alias) {
    if (!alias) {
      return;
    }
    this.resetVirtualModelForm();
    this.vmFormOpen = true;
    this.vmFormMode = "edit";
    this.vmFormSourceLocked = false;
    this.vmFormHasExisting = true;
    this.vmFormManaged = Boolean(alias.managed);
    this.vmFormOriginalSource = alias.name || "";
    this.vmFormDisplayName = alias.name || "";
    // A redirect has a single enabled flag; show it against the process-wide
    // default so a disabled alias reads "Effective now: no".
    this.vmFormDefaultEnabled = modelOverridesDefaultEnabled(modelsStore.models);
    this.vmFormEffectiveEnabled = alias.enabled !== false;

    const { primaryModel, primaryWeight, extraTargets } = aliasFormTargets(alias);
    this.vmForm = {
      source: alias.name || "",
      target_model: primaryModel,
      target_weight: primaryWeight,
      targets: extraTargets,
      strategy: alias.strategy || "round_robin",
      session_affinity: alias.session_affinity !== false,
      user_paths: (Array.isArray(alias.user_paths) ? alias.user_paths : []).join("\n"),
      description: alias.description || "",
      slowdown: alias.slowdown ?? "",
      enabled: alias.enabled !== false,
    };
  }

  // openVirtualModelEditModel edits the virtual model attached to a real
  // model row (or seeds a policy for it). Source is locked and prefilled with
  // the model selector.
  openVirtualModelEditModel(row) {
    if (!row || row.is_alias) {
      return;
    }
    const access = row.access || {};
    const override = access.override || null;
    const userPaths =
      override && Array.isArray(override.user_paths)
        ? override.user_paths
        : Array.isArray(access.user_paths)
          ? access.user_paths
          : [];
    const selector = rowAccessSelector(row);

    this.resetVirtualModelForm();
    this.vmFormOpen = true;
    this.vmFormMode = "edit";
    this.vmFormSourceLocked = true;
    this.vmFormHasExisting = Boolean(override);
    this.vmFormOriginalSource = selector;
    // Prefer the override's own enabled value; fall back to the backend
    // effective state only when no override row exists for this selector.
    const overrideEnabled = override
      ? override.enabled !== false
      : access.effective_enabled !== false;
    this.vmFormDefaultEnabled = access.default_enabled !== false;
    this.vmFormEffectiveEnabled = overrideEnabled;
    this.vmFormManaged = Boolean(override && override.managed);
    this.vmFormDisplayName = row.access_display_name || row.display_name || selector || "";
    this.vmForm = {
      source: selector,
      target_model: "",
      target_weight: "",
      targets: [],
      strategy: "round_robin",
      user_paths: userPaths.join("\n"),
      description: override && override.description ? override.description : "",
      slowdown: override && override.slowdown != null ? override.slowdown : "",
      enabled: overrideEnabled,
    };
    this.loadThinkingIntoForm(selector);
  }

  openGlobalModelOverrideEdit() {
    const selector = GLOBAL_OVERRIDE_SELECTOR;
    const override = this.findModelOverrideView(selector);
    const userPaths = override && Array.isArray(override.user_paths) ? override.user_paths : [];
    const defaultEnabled = modelOverridesDefaultEnabled(modelsStore.models);

    this.resetVirtualModelForm();
    this.vmFormOpen = true;
    this.vmFormMode = "edit";
    this.vmFormSourceLocked = true;
    this.vmFormHasExisting = Boolean(override);
    this.vmFormOriginalSource = selector;
    this.vmFormDefaultEnabled = defaultEnabled;
    this.vmFormEffectiveEnabled = override ? override.enabled !== false : defaultEnabled;
    this.vmFormManaged = Boolean(override && override.managed);
    this.vmFormDisplayName = t("vm.allProvidersModels");
    this.vmForm = {
      source: selector,
      target_model: "",
      target_weight: "",
      targets: [],
      strategy: "round_robin",
      user_paths: userPaths.join("\n"),
      description: override && override.description ? override.description : "",
      slowdown: override && override.slowdown != null ? override.slowdown : "",
      enabled: override ? override.enabled !== false : defaultEnabled,
    };
    this.loadThinkingIntoForm(selector);
  }

  openProviderOverrideEdit(group) {
    if (!group || !group.access || !group.access.selector) {
      return;
    }
    this.openVirtualModelEditModel({
      display_name: group.display_name,
      access_display_name: t("vm.allModelsIn", { name: group.display_name }),
      provider_name: group.provider_name,
      provider_type: group.provider_type,
      access: group.access,
      override_selector: group.access.selector,
      is_alias: false,
    });
  }

  // submitVirtualModelForm saves the unified editor: a filled target makes a
  // redirect/alias (several targets load balance by strategy), an empty one
  // makes an access policy.
  async submitVirtualModelForm() {
    if (this.vmFormManaged) {
      this.vmFormError = t("vm.managedCannotEdit");
      return;
    }

    const built = buildVirtualModelSavePayload(this.vmForm, this.vmFormOriginalSource, this.vmFormMode);
    const { payload, source, isRedirect, isRename } = built;

    if (!source) {
      this.vmFormError = t("vm.sourceRequired");
      return;
    }

    this.vmFormError = "";

    // In create mode warn about clobbering an existing virtual model (alias or
    // access policy) on the same source, or masking a concrete model. Edit
    // mode locks the source so none of these apply. The overwrite check must
    // run even with an empty target, since that is a policy upsert that can
    // still replace an existing redirect/policy row.
    if (this.vmFormMode !== "edit") {
      const existingAlias = this.findExistingAliasByName(source);
      const existingPolicy = existingAlias ? null : this.findModelOverrideView(source);
      if (existingAlias || existingPolicy) {
        const overwriteMessage = existingAlias
          ? 'A virtual model named "' +
            existingAlias.name +
            '" already exists. Saving will update that virtual model. Continue?'
          : 'An access policy for "' +
            source +
            '" already exists. Saving will update that virtual model. Continue?';
        if (!window.confirm(overwriteMessage)) {
          this.vmFormError = "Choose a different source or edit the existing virtual model.";
          return;
        }
      } else if (isRedirect) {
        const matchingModel = this.findConcreteModelByName(source);
        if (matchingModel) {
          const modelName =
            qualifiedModelName(matchingModel) ||
            String((matchingModel.model && matchingModel.model.id) || "").trim();
          if (
            !window.confirm(
              'A model named "' +
                modelName +
                '" already exists. Creating this alias will mask that model in the list. Continue?',
            )
          ) {
            this.vmFormError = "Choose a different source to avoid masking an existing model.";
            return;
          }
        }
      }
    } else if (isRename) {
      // Renaming: the backend refuses to clobber an existing row, so block
      // early with a clear message instead of overwriting another virtual
      // model. Match the source exactly (case-sensitive) like the backend key,
      // so a case-only rename (e.g. "Smart" -> "smart") is not wrongly
      // rejected. Masking a concrete model is still a soft confirm.
      const existingAlias = (this.aliases || []).find((entry) => entry && entry.name === source) || null;
      const existingPolicy = existingAlias ? null : this.findModelOverrideView(source);
      if (existingAlias || existingPolicy) {
        this.vmFormError = 'A virtual model for "' + source + '" already exists. Choose a different source.';
        return;
      }
      if (isRedirect) {
        const matchingModel = this.findConcreteModelByName(source);
        if (matchingModel) {
          const modelName =
            qualifiedModelName(matchingModel) ||
            String((matchingModel.model && matchingModel.model.id) || "").trim();
          if (
            !window.confirm(
              'A model named "' +
                modelName +
                '" already exists. Renaming to that name will mask the model in the list. Continue?',
            )
          ) {
            this.vmFormError = "Choose a different source to avoid masking an existing model.";
            return;
          }
        }
      }
    }

    this.vmSubmitting = true;

    try {
      const result = await sendJSON("/admin/virtual-models", "PUT", payload, {
        label: "virtual model",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        this.vmFormError = "Virtual models feature is unavailable.";
        return;
      }
      if (result.stale) {
        return;
      }
      if (!result.ok) {
        this.vmFormError =
          result.status === 401
            ? "Authentication required."
            : errorMessage(result, "Failed to save virtual model.");
        return;
      }
      const policyPruned = !isRedirect && result.status === 204;
      this.virtualModelsAvailable = true;

      const thinkingSelector = String(this.vmForm.source || this.vmFormOriginalSource || "").trim();
      if (thinkingFormVisibleForSource(thinkingSelector)) {
        await this.saveThinkingOverride(thinkingSelector);
      }

      this.closeVirtualModelForm();
      flash.success(
        isRedirect
          ? "Alias saved."
          : policyPruned
            ? "Model access reset to inherited/default."
            : "Model access saved.",
      );
      void Promise.all([modelsStore.fetchModels(), this.fetchVirtualModels()]);
    } catch (e) {
      console.error("Failed to save virtual model:", e);
      this.vmFormError = "Failed to save virtual model.";
    } finally {
      this.vmSubmitting = false;
    }
  }

  async saveThinkingOverride(selector) {
    const mode = String(this.vmFormThinking.mode || "native").trim();
    const hadOverride = Boolean(this.thinkingOverrideFor(selector));
    try {
      if (mode === "native") {
        if (hadOverride) {
          await sendJSON("/admin/model-thinking-overrides", "DELETE", { selector }, {
            label: "thinking override",
          });
        }
      } else {
        await sendJSON(
          "/admin/model-thinking-overrides",
          "PUT",
          {
            selector,
            mode,
            thinking: this.vmFormThinking.thinking !== false,
            effort: String(this.vmFormThinking.effort || "medium").trim(),
          },
          { label: "thinking override" },
        );
      }
      void this.fetchThinkingOverrides();
    } catch (e) {
      console.error("Failed to save thinking override:", e);
      flash.error("Model saved, but the thinking override could not be saved.");
    }
  }

  // deleteVirtualModel removes the virtual model for the editor's source.
  async deleteVirtualModel() {
    if (this.vmFormManaged) {
      this.vmFormError = "This virtual model is managed by configuration and cannot be removed here.";
      return;
    }
    const source = String(this.vmForm.source || this.vmFormOriginalSource || "").trim();
    if (!source || !this.vmFormHasExisting) {
      return;
    }
    if (
      !window.confirm(
        'Remove the virtual model for "' + source + '"? This reverts to inherited/default behavior.',
      )
    ) {
      return;
    }

    this.vmDeleting = true;
    this.vmFormError = "";

    try {
      const result = await sendJSON("/admin/virtual-models", "DELETE", { source }, {
        label: "virtual model",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        this.vmFormError = "Virtual models feature is unavailable.";
        return;
      }
      if (result.status !== 404) {
        if (result.stale) {
          return;
        }
        if (!result.ok) {
          this.vmFormError =
            result.status === 401
              ? "Authentication required."
              : errorMessage(result, "Failed to remove virtual model.");
          return;
        }
      }
      this.virtualModelsAvailable = true;

      this.closeVirtualModelForm();
      flash.success("Virtual model removed.");

      const delSelector = String(this.vmForm.source || this.vmFormOriginalSource || "").trim();
      if (thinkingFormVisibleForSource(delSelector) && this.thinkingOverrideFor(delSelector)) {
        try {
          await sendJSON("/admin/model-thinking-overrides", "DELETE", { selector: delSelector }, {
            label: "thinking override",
          });
          void this.fetchThinkingOverrides();
        } catch (e) {
          console.error("Failed to delete thinking override:", e);
        }
      }
      void Promise.all([modelsStore.fetchModels(), this.fetchVirtualModels()]);
    } catch (e) {
      console.error("Failed to delete virtual model:", e);
      this.vmFormError = "Failed to remove virtual model.";
    } finally {
      this.vmDeleting = false;
    }
  }
}

export const virtualModels = new VirtualModelsStore();
