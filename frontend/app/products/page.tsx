"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";
import { Package, Plus, CheckCircle, XCircle } from "lucide-react";

interface Product {
  id: string;
  sku: string;
  name: string;
  price: number;
  active: boolean;
}

export default function ProductsPage() {
  const { t } = useI18n();
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({ sku: "", name: "", price: 0, active: true });
  const [error, setError] = useState("");

  const { data: products, loading, reload: loadProducts } = useResource(
    async () => (await api.getProducts()) || [],
    {
      initial: [] as Product[],
      onError: (err: any) => setError(err.message || t("products.message.load_failed")),
    },
  );

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.createProduct(form);
      setShowModal(false);
      setForm({ sku: "", name: "", price: 0, active: true });
      await loadProducts();
    } catch (err: any) {
      setError(err.message || t("products.message.create_failed"));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-slate-200 pb-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
            <Package className="w-6 h-6 text-emerald-600" />
            <span>{t("products.view.title")}</span>
          </h1>
          <p className="text-sm text-slate-500">{t("products.view.subtitle")}</p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="bg-emerald-600 hover:bg-emerald-700 text-white font-medium text-sm py-2 px-4 rounded-lg flex items-center space-x-2 transition"
        >
          <Plus className="w-4 h-4" />
          <span>{t("products.action.create")}</span>
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg">
          {error}
        </div>
      )}

      {loading ? (
        <div className="py-8 text-slate-500 text-sm">{t("products.message.loading")}</div>
      ) : products.length === 0 ? (
        <div className="bg-white border border-slate-200 rounded-xl p-12 text-center text-slate-500 text-sm">
          {t("products.message.empty")}
        </div>
      ) : (
        <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse text-sm">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-200 text-slate-600 font-semibold text-xs uppercase tracking-wider">
                <th className="py-3 px-4">{t("products.field.sku")}</th>
                <th className="py-3 px-4">{t("products.field.name")}</th>
                <th className="py-3 px-4">{t("products.field.price")}</th>
                <th className="py-3 px-4">{t("base.field.status")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {products.map((p) => (
                <tr key={p.id} className="hover:bg-slate-50">
                  <td className="py-3.5 px-4 font-mono text-xs font-bold text-indigo-600">{p.sku}</td>
                  <td className="py-3.5 px-4 font-bold text-slate-900">{p.name}</td>
                  <td className="py-3.5 px-4 font-semibold text-slate-800">${p.price.toFixed(2)}</td>
                  <td className="py-3.5 px-4">
                    <span
                      className={`inline-flex items-center space-x-1 px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                        p.active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-600"
                      }`}
                    >
                      {p.active ? <CheckCircle className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                      <span>{p.active ? "Active" : "Inactive"}</span>
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Modal */}
      {showModal && (
        <Modal label={t("products.view.create_title")}>
          <h2 className="text-xl font-bold text-slate-900 mb-4">{t("products.view.create_title")}</h2>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">SKU *</label>
              <input
                type="text"
                placeholder={t("products.field.sku_placeholder")}
                value={form.sku}
                onChange={(e) => setForm({ ...form, sku: e.target.value })}
                className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-emerald-500 font-mono"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("products.field.name")} *</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-emerald-500"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">Price ($) *</label>
              <input
                type="number"
                step="0.01"
                value={form.price}
                onChange={(e) => setForm({ ...form, price: parseFloat(e.target.value) || 0 })}
                className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-emerald-500"
                required
              />
            </div>

            <div className="flex items-center space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="w-1/2 bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium py-2 rounded-lg text-sm"
              >
                Cancel
              </button>
              <button
                type="submit"
                className="w-1/2 bg-emerald-600 hover:bg-emerald-700 text-white font-medium py-2 rounded-lg text-sm"
              >
                {t("products.action.save")}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
