"use client";

import React, { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { AdminOnly, useAccess } from "@/lib/permissions";
import { Share2, Plus, CheckCircle2, ShieldAlert, Activity, Globe, RefreshCw } from "lucide-react";

export default function IntegrationsPage() {
  const { t } = useI18n();
  const [integrations, setIntegrations] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const { loading: checking, isAdmin } = useAccess();
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({
    name: "",
    type: "webhook",
    target_url: "",
    secret_key: "",
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const data = await api.getIntegrations();
      setIntegrations(data || []);
    } catch (err) {
      // Swallowing this rendered an empty list, which reads as "you have no
      // integrations" to a member who simply is not allowed to see them.
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.registerIntegration(form);
      setShowModal(false);
      setForm({ name: "", type: "webhook", target_url: "", secret_key: "" });
      loadData();
    } catch (err: any) {
      alert("Failed to register integration: " + err.message);
    }
  };

  // The endpoints behind this screen are administrator-only, so a member
  // without those rights is told as much rather than shown an empty list.
  if (!checking && !isAdmin) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
          <Share2 className="w-7 h-7 text-indigo-600" />
          <span>{t("integrations.view.title")}</span>
        </h1>
        <AdminOnly />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {error && (
        <p className="text-sm text-rose-700 bg-rose-50 border border-rose-200 rounded-lg px-3 py-2">
          {error}
        </p>
      )}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
            <Share2 className="w-7 h-7 text-indigo-600" />
            <span>{t("integrations.view.title")}</span>
          </h1>
          <p className="text-sm text-slate-500 mt-1">
            Manage government gateways, REST API connectors, and event webhooks
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <button
            onClick={loadData}
            className="p-2 text-slate-600 hover:bg-slate-100 rounded-lg border border-slate-200 transition"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowModal(true)}
            className="bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center space-x-2 shadow-sm transition"
          >
            <Plus className="w-4 h-4" />
            <span>{t("integrations.action.create")}</span>
          </button>
        </div>
      </div>

      {loading ? (
        <div className="py-12 text-center text-slate-400">{t("integrations.message.loading")}</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {integrations.map((item) => (
            <div
              key={item.id}
              className="bg-white p-5 rounded-xl border border-slate-200 shadow-sm flex flex-col justify-between"
            >
              <div>
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center space-x-3">
                    <div className="p-2.5 bg-indigo-50 text-indigo-600 rounded-lg">
                      <Globe className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="font-bold text-slate-900 text-base">{item.name}</h3>
                      <span className="text-[11px] font-mono bg-slate-100 text-slate-600 px-2 py-0.5 rounded">
                        {item.type}
                      </span>
                    </div>
                  </div>
                  <span
                    className={`inline-flex items-center space-x-1 text-xs font-bold px-2.5 py-1 rounded-full ${
                      item.status === "ACTIVE"
                        ? "bg-emerald-50 text-emerald-700 border border-emerald-200"
                        : "bg-amber-50 text-amber-700 border border-amber-200"
                    }`}
                  >
                    {item.status === "ACTIVE" ? (
                      <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
                    ) : (
                      <ShieldAlert className="w-3.5 h-3.5 text-amber-500" />
                    )}
                    <span>{item.status}</span>
                  </span>
                </div>

                <div className="text-xs text-slate-500 space-y-1 font-mono bg-slate-50 p-2.5 rounded-lg border border-slate-100">
                  <div className="truncate">URL: {item.target_url}</div>
                  <div>Last Health Ping: {new Date(item.last_ping_at).toLocaleTimeString()}</div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add Integration Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-slate-900/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl border border-slate-200">
            <h2 className="text-xl font-bold text-slate-900 mb-4">{t("integrations.view.create_title")}</h2>
            <form onSubmit={handleRegister} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Integration Name *</label>
                <input
                  type="text"
                  placeholder={t("integrations.field.name_placeholder")}
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">{t("integrations.field.type")}</label>
                <select
                  value={form.type}
                  onChange={(e) => setForm({ ...form, type: e.target.value })}
                  className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                >
                  <option value="webhook">{t("integrations.type.webhook")}</option>
                  <option value="government">{t("integrations.type.government_gateway")}</option>
                  <option value="payment">{t("integrations.type.payment_gateway")}</option>
                  <option value="custom_rest">{t("integrations.type.custom_rest")}</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Target Endpoint URL *</label>
                <input
                  type="url"
                  placeholder="https://api.example.com/webhooks"
                  value={form.target_url}
                  onChange={(e) => setForm({ ...form, target_url: e.target.value })}
                  className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">{t("integrations.field.secret")}</label>
                <input
                  type="password"
                  placeholder={t("integrations.field.secret_placeholder")}
                  value={form.secret_key}
                  onChange={(e) => setForm({ ...form, secret_key: e.target.value })}
                  className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div className="flex items-center space-x-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="w-1/2 bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium py-2 rounded-lg text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="w-1/2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium py-2 rounded-lg text-xs"
                >
                  Register Connector
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
