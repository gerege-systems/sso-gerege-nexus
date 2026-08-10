"use client";

import React, { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Settings, Clock } from "lucide-react";

interface InstalledApp {
  id: string;
  app_id: string;
  slug: string;
  name: string;
  installed_version: string;
  status: string;
  enabled: boolean;
  installed_at: string;
}

export default function InstalledAppsSettingsPage() {
  const { t, locale } = useI18n();
  const [apps, setApps] = useState<InstalledApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const loadInstalled = async () => {
    try {
      const data = await api.getInstalledApps();
      setApps(data || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setLoading(true);
    loadInstalled();
  }, [locale]);

  const handleToggle = async (app: InstalledApp) => {
    setActionLoading(app.slug);
    try {
      if (app.enabled) {
        await api.disableApp(app.slug);
      } else {
        await api.enableApp(app.slug);
      }
      await loadInstalled();
    } catch (err) {
      console.error(err);
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="border-b border-slate-200 pb-4">
        <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
          <Settings className="w-6 h-6 text-slate-600" />
          <span>{t("app_store.view.installed_title")}</span>
        </h1>
        <p className="text-sm text-slate-500">
          {t("app_store.view.installed_subtitle")}
        </p>
      </div>

      {loading ? (
        <div className="py-8 text-slate-500 text-sm">{t("app_store.message.loading_installed")}</div>
      ) : apps.length === 0 ? (
        <div className="bg-white border border-slate-200 rounded-xl p-8 text-center text-slate-500 text-sm">
          {t("app_store.message.none_installed")}{" "}
          <a href="/apps" className="text-indigo-600 font-semibold underline">
            {t("app_store.action.browse_store")}
          </a>
        </div>
      ) : (
        <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse text-sm">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-200 text-slate-600 font-semibold text-xs uppercase tracking-wider">
                <th className="py-3 px-4">{t("app_store.field.application_name")}</th>
                <th className="py-3 px-4">{t("app_store.field.module_id")}</th>
                <th className="py-3 px-4">{t("app_store.field.installed_version")}</th>
                <th className="py-3 px-4">{t("base.field.status")}</th>
                <th className="py-3 px-4">{t("app_store.field.installed_date")}</th>
                <th className="py-3 px-4 text-right">{t("base.field.actions")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {apps.map((app) => (
                <tr key={app.id} className="hover:bg-slate-50">
                  <td className="py-3.5 px-4 font-bold text-slate-900">{app.name}</td>
                  <td className="py-3.5 px-4 font-mono text-xs text-slate-500">{app.app_id}</td>
                  <td className="py-3.5 px-4 font-semibold text-slate-700">v{app.installed_version}</td>
                  <td className="py-3.5 px-4">
                    <span
                      className={`inline-flex items-center space-x-1.5 px-2.5 py-1 rounded-full text-xs font-semibold ${
                        app.enabled
                          ? "bg-emerald-50 text-emerald-700 border border-emerald-200"
                          : "bg-slate-100 text-slate-600 border border-slate-200"
                      }`}
                    >
                      <span className={`w-1.5 h-1.5 rounded-full ${app.enabled ? "bg-emerald-500" : "bg-slate-400"}`}></span>
                      <span>{app.enabled ? "Active" : "Disabled"}</span>
                    </span>
                  </td>
                  <td className="py-3.5 px-4 text-xs text-slate-500 flex items-center space-x-1 pt-4">
                    <Clock className="w-3.5 h-3.5" />
                    <span>{new Date(app.installed_at).toLocaleDateString()}</span>
                  </td>
                  <td className="py-3.5 px-4 text-right">
                    <button
                      onClick={() => handleToggle(app)}
                      disabled={actionLoading === app.slug}
                      className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition border ${
                        app.enabled
                          ? "border-red-200 text-red-600 hover:bg-red-50"
                          : "border-emerald-600 bg-emerald-600 text-white hover:bg-emerald-700"
                      }`}
                    >
                      {app.enabled ? "Disable" : "Enable"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
