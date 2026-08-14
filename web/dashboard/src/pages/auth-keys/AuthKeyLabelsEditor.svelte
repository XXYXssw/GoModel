<script>
  // Edit-labels modal for an existing API key (EditorDialog shell).
  import EditorDialog from "$lib/components/organisms/EditorDialog.svelte";
  import FormField from "$lib/components/molecules/FormField.svelte";
  import { authKeysStore as store } from "./authKeys.svelte.js";
  import { t } from "$lib/i18n/i18n.svelte.js";
</script>

<EditorDialog
  open={store.labelsEditor.open}
  title={t("authKeys.editLabels")}
  ariaLabel={t("authKeys.labelsEditorAria")}
  error={store.labelsEditor.error}
  submitting={store.labelsEditor.submitting}
  submitLabel={t("authKeys.saveLabels")}
  cancel={false}
  dialogClass="auth-key-editor"
  onclose={() => store.closeLabelsEditor()}
  onsubmit={() => store.submitLabelsEditor()}
>
  {#snippet headerHint()}
    <p class="form-hint">{store.labelsEditor.name}</p>
  {/snippet}

  <FormField id="auth-key-labels-edit" label={t("authKeys.labelsField")}>
    <input
      id="auth-key-labels-edit"
      type="text"
      placeholder={t("authKeys.labelsPlaceholder")}
      autocomplete="off"
      data-modal-autofocus
      bind:value={store.labelsEditor.value}
    />
    <p class="form-hint">
      Applies to new requests made with this key. Leave empty to remove all labels.
    </p>
  </FormField>
</EditorDialog>
