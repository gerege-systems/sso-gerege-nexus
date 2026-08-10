"use client";

import React, { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner } from "@/components/ui";
import {
  Search,
  Download,
  Power,
  PowerOff,
  Boxes,
  Users,
  Package,
} from "lucide-react";

interface AppItem {
  id: string;
  slug: string;
  name: string;
  description: string;
  icon_url: string;
  category: string;
  version: string;
  installed: boolean;
  enabled: boolean;
  manifest: {
    dependencies?: Array<{ id: string; version_constraint: string }>;
  };
}

const appIcons: Record<string, React.ReactNode> = {
  contacts: <Users className="w-8 h-8 text-indigo-500" />,
  products: <Package className="w-8 h-8 text-emerald-500" />,
  inventory: <Boxes className="w-8 h-8 text-amber-500" />,
};

export default function AppStorePage() {
  const { t } = useI18n();
  const [apps, setApps] = useState<AppItem[]>([]);
  const [search, setSearch] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("All");
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const loadApps = useCallback(async () => {
    try {
      const data = await api.getStoreApps();
      setApps(data || []);
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("app_store.message.load_failed") });
    } finally {
      setLoading(false);
    }
  }, [t]);

  // The store's copy is translated by the API, so a language change means
  // fetching the catalogue again rather than re-rendering what is held. `t`
  // changes identity with the locale and nothing else, so depending on loadApps
  // says exactly that — the effect used to name `locale` and quietly leave out
  // the function it calls.
  useEffect(() => {
    setLoading(true);
    setSelectedCategory("All");
    void loadApps();
  }, [loadApps]);

  const handleInstall = async (app: AppItem) => {
    setActionLoading(app.slug);
    setMessage(null);
    try {
      await api.installApp(app.slug);
      setMessage({
        type: "success",
        text: `Successfully installed ${app.name} and its dependencies!`,
      });
      await loadApps();
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || `Failed to install ${app.name}` });
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggleState = async (app: AppItem) => {
    setActionLoading(app.slug);
    setMessage(null);
    try {
      if (app.enabled) {
        await api.disableApp(app.slug);
        setMessage({ type: "success", text: `Disabled ${app.name}` });
      } else {
        await api.enableApp(app.slug);
        setMessage({ type: "success", text: `Enabled ${app.name}` });
      }
      await loadApps();
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("app_store.message.action_failed") });
    } finally {
      setActionLoading(null);
    }
  };

  const categories = ["All", ...Array.from(new Set(apps.map((a) => a.category)))];

  const filteredApps = apps.filter((app) => {
    const matchesSearch =
      app.name.toLowerCase().includes(search.toLowerCase()) ||
      app.description.toLowerCase().includes(search.toLowerCase());
    const matchesCategory = selectedCategory === "All" || app.category === selectedCategory;
    return matchesSearch && matchesCategory;
  });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 pb-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t("app_store.view.title")}</h1>
          <p className="text-sm text-slate-500">{t("app_store.view.subtitle")}</p>
        </div>

        {/* Search & Category */}
        <div className="flex items-center space-x-3">
          <div className="relative">
            <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-400" />
            <input
              type="text"
              placeholder={t("app_store.view.search_placeholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 pr-3 py-1.5 text-sm border border-slate-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 w-64 bg-white"
            />
          </div>

          <select
            value={selectedCategory}
            onChange={(e) => setSelectedCategory(e.target.value)}
            className="px-3 py-1.5 text-sm border border-slate-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 bg-white"
          >
            {categories.map((c) => (
              <option key={c} value={c}>
                {c === "All" ? t("app_store.filter.all") : c}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Notifications */}
      {message && (
        <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />
      )}

      {/* App Cards Grid */}
      {loading ? (
        <div className="py-12 text-center text-slate-500 text-sm">{t("app_store.message.loading")}</div>
      ) : filteredApps.length === 0 ? (
        <div className="py-12 text-center text-slate-500 text-sm">{t("app_store.message.no_match")}</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredApps.map((app) => (
            <div
              key={app.id}
              className="bg-white border border-slate-200 rounded-xl p-5 shadow-sm hover:shadow-md transition flex flex-col justify-between"
            >
              <div>
                <div className="flex items-start justify-between mb-3">
                  <div className="p-2.5 bg-slate-50 rounded-xl border border-slate-100">
                    {appIcons[app.slug] || <Boxes className="w-8 h-8 text-indigo-500" />}
                  </div>
                  <div className="flex items-center space-x-2">
                    <span className="text-xs bg-slate-100 text-slate-600 font-semibold px-2 py-0.5 rounded">
                      v{app.version}
                    </span>
                    {app.installed && (
                      <span
                        className={`text-xs font-semibold px-2 py-0.5 rounded ${
                          app.enabled
                            ? "bg-emerald-100 text-emerald-700"
                            : "bg-amber-100 text-amber-700"
                        }`}
                      >
                        {app.enabled ? t("app_store.state.installed") : t("app_store.state.disabled")}
                      </span>
                    )}
                  </div>
                </div>

                <h2 className="text-lg font-bold text-slate-900">{app.name}</h2>
                <p className="text-xs font-medium text-indigo-600 mb-2">{app.category}</p>
                <p className="text-sm text-slate-600 mb-4 line-clamp-2">{app.description}</p>

                {/* Dependencies info */}
                {app.manifest.dependencies && app.manifest.dependencies.length > 0 && (
                  <div className="mb-4 text-xs text-slate-500 bg-slate-50 p-2.5 rounded-lg border border-slate-100">
                    <span className="font-semibold text-slate-700">{t("app_store.field.requires")}</span>
                    {app.manifest.dependencies.map((d) => d.id.replace("io.example.", "")).join(", ")}
                  </div>
                )}
              </div>

              {/* Action Buttons */}
              <div className="pt-3 border-t border-slate-100 flex items-center justify-end space-x-2">
                {!app.installed ? (
                  <button
                    onClick={() => handleInstall(app)}
                    disabled={actionLoading === app.slug}
                    className="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-sm py-2 px-4 rounded-lg flex items-center justify-center space-x-2 transition disabled:opacity-50"
                  >
                    <Download className="w-4 h-4" />
                    <span>{actionLoading === app.slug ? "Installing..." : "Install App"}</span>
                  </button>
                ) : (
                  <button
                    onClick={() => handleToggleState(app)}
                    disabled={actionLoading === app.slug}
                    className={`w-full font-medium text-sm py-2 px-4 rounded-lg flex items-center justify-center space-x-2 transition border ${
                      app.enabled
                        ? "bg-white hover:bg-red-50 text-red-600 border-red-200"
                        : "bg-emerald-600 hover:bg-emerald-700 text-white border-emerald-600"
                    }`}
                  >
                    {app.enabled ? (
                      <>
                        <PowerOff className="w-4 h-4" />
                        <span>{t("app_store.action.disable")}</span>
                      </>
                    ) : (
                      <>
                        <Power className="w-4 h-4" />
                        <span>{t("app_store.action.enable")}</span>
                      </>
                    )}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
