<script>
  // MCP Servers page: list, filter, create/edit, reconnect, delete, and the
  // per-server catalog inspector. The server list is fetched when this page
  // is active (the overview card does its own count fetch) and again
  // whenever the API key changes.
  import LoadingState from "$lib/components/molecules/LoadingState.svelte";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import FilterInput from "$lib/components/molecules/FilterInput.svelte";
  import InlineHelpSection from "$lib/components/molecules/InlineHelpSection.svelte";
  import { auth } from "$lib/stores/auth.svelte.js";
  import { router } from "$lib/stores/router.svelte.js";
  import McpCatalogModal from "./McpCatalogModal.svelte";
  import McpServerEditor from "./McpServerEditor.svelte";
  import McpServerList from "./McpServerList.svelte";
  import { mcpServers } from "./mcpServers.svelte.js";
  import { Plus } from "lucide";
  import { t } from "$lib/i18n/i18n.svelte.js";

  const PAGE = "mcp-servers";
  const HELP_TEXT =
    "Upstream Model Context Protocol servers whose tools, prompts, and resources the gateway exposes to clients. Servers added here connect over HTTP or SSE; stdio servers and rows marked Config are declared in config.yaml under mcp.servers and are read-only in the dashboard. Saved header values are masked in API and dashboard responses.";

  // Re-fetch when the page becomes active or the API key changes.
  $effect(() => {
    void auth.refreshTick;
    if (router.page === PAGE) mcpServers.fetchServers();
  });
</script>

<div>
  <div class="page-header">
    <div>
      <InlineHelpSection copyId="mcp-servers-help-copy" label={t("mcp.helpLabel")} text={HELP_TEXT}>
        {#snippet title()}<h2>{t("mcp.title")}</h2>{/snippet}
      </InlineHelpSection>
    </div>
    <div class="page-header-controls">
      {#if mcpServers.available && !auth.authError}
        <button
          type="button"
          class="btn btn-primary btn-with-icon"
          disabled={mcpServers.formSubmitting}
          onclick={() => mcpServers.openCreate()}
        >
          <Icon icon={Plus} class="form-action-icon" />
          <span>{t("mcp.addServer")}</span>
        </button>
      {/if}
    </div>
  </div>

  {#if !mcpServers.available && !auth.authError}
    <div class="alert alert-warning">{t("mcp.unavailable")}</div>
  {/if}
  {#if mcpServers.error && !auth.authError && !mcpServers.formOpen}
    <p class="form-error" role="alert" aria-live="assertive">{mcpServers.error}</p>
  {/if}
  {#if mcpServers.loading && !auth.authError}
    <LoadingState label={t("mcp.loading")} />
  {/if}

  {#if (mcpServers.servers.length > 0 || mcpServers.filter) && mcpServers.available && !auth.authError}
    <div class="table-toolbar">
      <div class="table-toolbar-main">
        <FilterInput
          id="mcp-server-filter"
          placeholder={t("mcp.filterPlaceholder")}
          label={t("mcp.filterLabel")}
          bind:value={mcpServers.filter}
        />
      </div>
    </div>
  {/if}

  <McpServerEditor />
  <McpCatalogModal />

  {#if mcpServers.filtered.length > 0 && mcpServers.available && !auth.authError}
    <McpServerList />
  {/if}

  {#if mcpServers.servers.length === 0 && !mcpServers.filter && !mcpServers.loading && !auth.authError && !mcpServers.error && mcpServers.available}
    <p class="empty-state">
      No MCP servers yet. Add one here, or declare servers in <code>config.yaml</code> under
      <code>mcp.servers</code>.
    </p>
  {/if}
  {#if mcpServers.servers.length > 0 && mcpServers.filtered.length === 0 && mcpServers.filter && !mcpServers.loading && !auth.authError && mcpServers.available}
    <p class="empty-state">{t("mcp.emptyFilter")}</p>
  {/if}
</div>
