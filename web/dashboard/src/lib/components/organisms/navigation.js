// Sidebar navigation registry: the ordered list of dashboard pages with
// their lucide icons and optional runtime-config visibility gates.
// Each item is { page, labelKey, icon, visible? }: `page` is the route id
// under /admin/dashboard/{page} (see $lib/stores/router), `labelKey` the
// i18n key for the page label, `icon` a lucide icon imported below and
// passed to the Icon atom, and `visible` a feature gate — it reads the
// runtimeConfig runes store, so call it inside a reactive context
// (Sidebar's $derived) to re-filter when flags load.

import { runtimeConfig } from "$lib/stores/runtimeConfig.svelte.js";
import {
  Box,
  ChartColumn,
  Gauge,
  History,
  KeyRound,
  LayoutDashboard,
  Plug,
  ServerCog,
  Settings,
  ShieldCheck,
  Wallet,
  Workflow,
} from "lucide";

export const NAV_ITEMS = [
  { page: "overview", labelKey: "nav.overview", icon: LayoutDashboard },
  { page: "providers-config", labelKey: "nav.providers", icon: ServerCog },
  { page: "models", labelKey: "nav.models", icon: Box },
  { page: "audit-logs", labelKey: "nav.auditLogs", icon: History },
  { page: "usage", labelKey: "nav.usage", icon: ChartColumn },
  {
    page: "budgets",
    labelKey: "nav.budgets",
    icon: Wallet,
    visible: () => runtimeConfig.budgetsVisible(),
  },
  {
    page: "rate-limits",
    labelKey: "nav.rateLimits",
    icon: Gauge,
    visible: () => runtimeConfig.rateLimitsVisible(),
  },
  { page: "auth-keys", labelKey: "nav.apiKeys", icon: KeyRound },
  { page: "workflows", labelKey: "nav.workflows", icon: Workflow },
  {
    page: "guardrails",
    labelKey: "nav.guardrails",
    icon: ShieldCheck,
    visible: () => runtimeConfig.guardrailsVisible(),
  },
  {
    page: "mcp-servers",
    labelKey: "nav.mcpServers",
    icon: Plug,
    visible: () => runtimeConfig.mcpVisible(),
  },
  { page: "settings", labelKey: "nav.settings", icon: Settings },
];
