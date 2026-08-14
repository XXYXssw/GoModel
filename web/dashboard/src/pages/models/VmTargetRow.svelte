<script>
  // One target row of the virtual-model editor. The primary target and the
  // extra {#each} rows share this shape; two or more rows make the redirect
  // a load balancer (weights show only for round-robin).
  import Icon from "$lib/components/atoms/Icon.svelte";
  import { t } from "$lib/i18n/i18n.svelte.js";
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import { virtualModels as vm } from "./virtualModels.svelte.js";
  import { Trash2 } from "lucide";

  let {
    model = $bindable(""),
    weight = $bindable(),
    id = undefined,
    placeholder = "openai/gpt-4o",
    showRemove = true,
    onremove,
  } = $props();
</script>

<div class="vm-target-row">
  <input
    {id}
    type="text"
    list="virtual-model-target-options"
    class="mono vm-target-model"
    {placeholder}
    bind:value={model}
    disabled={vm.vmFormManaged}
    aria-label={t("vm.targetModel")}
  />
  {#if vm.vmFormShowWeights()}
    <input
      type="number"
      min="1"
      step="1"
      class="mono vm-target-weight"
      placeholder="weight"
      bind:value={weight}
      disabled={vm.vmFormManaged}
      required
      aria-label={t("vm.targetWeight")}
    />
  {/if}
  {#if showRemove}
    <TableActionButton
      label={t("vm.removeTarget")}
      class="table-action-btn-danger table-icon-btn vm-target-remove"
      onclick={onremove}
      disabled={vm.vmFormManaged}
    >
      <Icon icon={Trash2} class="table-icon-svg" />
    </TableActionButton>
  {/if}
</div>
