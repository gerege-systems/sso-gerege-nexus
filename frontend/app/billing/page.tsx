"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useI18n } from "@/lib/i18n";
import { LoadingBlock, Modal, TableCard, fieldClass } from "@/components/ui";
import { CreditCard, Plus, Receipt, CheckCircle, Clock } from "lucide-react";

export default function BillingPage() {
  const { t } = useI18n();
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({ contact_name: "", amount: "" });

  const { data: invoices, loading, reload: loadData } = useResource(
    async () => (await api.getInvoices()) || [],
    { initial: [] as any[] },
  );

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.createInvoice({
        contact_name: form.contact_name,
        amount: parseFloat(form.amount),
      });
      setShowModal(false);
      setForm({ contact_name: "", amount: "" });
      loadData();
    } catch (err: any) {
      alert(t("billing.message.create_failed") + ": " + err.message);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
            <CreditCard className="w-7 h-7 text-indigo-600" />
            <span>{t("billing.view.title")}</span>
          </h1>
          <p className="text-sm text-slate-500 mt-1">{t("billing.view.subtitle")}</p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center space-x-2 shadow-sm transition"
        >
          <Plus className="w-4 h-4" />
          <span>{t("billing.action.create")}</span>
        </button>
      </div>

      {loading ? (
        <LoadingBlock label={t("billing.message.loading")} />
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("billing.field.invoice_number")}</th>
              <th className="px-4 py-3">{t("billing.field.contact")}</th>
              <th className="px-4 py-3">{t("billing.field.total")}</th>
              <th className="px-4 py-3">10% VAT</th>
              <th className="px-4 py-3">{t("billing.field.ebarimt_status")}</th>
              <th className="px-4 py-3">{t("billing.field.payment_status")}</th>
            </tr>
          }
        >
          {invoices.map((inv) => (
            <tr key={inv.id} className="hover:bg-slate-50">
              <td className="px-4 py-3 font-mono font-semibold text-slate-900">{inv.invoice_number}</td>
              <td className="px-4 py-3 font-medium text-slate-800">{inv.contact_name}</td>
              <td className="px-4 py-3 font-bold text-indigo-600">₮{inv.amount?.toLocaleString()}</td>
              <td className="px-4 py-3 text-slate-500">₮{inv.vat_amount?.toLocaleString()}</td>
              <td className="px-4 py-3">
                <span className="bg-blue-50 text-blue-700 text-[11px] font-semibold px-2.5 py-0.5 rounded-full border border-blue-200 flex items-center w-max space-x-1">
                  <Receipt className="w-3 h-3 text-blue-500" />
                  <span>{inv.ebarimt_status}</span>
                </span>
              </td>
              <td className="px-4 py-3">
                <span
                  className={`inline-flex items-center space-x-1 text-[11px] font-bold px-2.5 py-0.5 rounded-full ${
                    inv.status === "PAID"
                      ? "bg-emerald-50 text-emerald-700 border border-emerald-200"
                      : "bg-amber-50 text-amber-700 border border-amber-200"
                  }`}
                >
                  {inv.status === "PAID" ? (
                    <CheckCircle className="w-3 h-3 text-emerald-500" />
                  ) : (
                    <Clock className="w-3 h-3 text-amber-500" />
                  )}
                  <span>{inv.status}</span>
                </span>
              </td>
            </tr>
          ))}
        </TableCard>
      )}

      {/* Modal */}
      {showModal && (
        <Modal label={t("billing.view.create_title")}>
          <h2 className="text-xl font-bold text-slate-900 mb-4">{t("billing.view.create_title")}</h2>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("billing.field.contact")} *</label>
              <input
                type="text"
                placeholder={t("billing.field.contact_placeholder")}
                value={form.contact_name}
                onChange={(e) => setForm({ ...form, contact_name: e.target.value })}
                className={fieldClass}
                required
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("billing.field.amount")} *</label>
              <input
                type="number"
                placeholder="150000"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
                className={fieldClass}
                required
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
                {t("billing.action.generate")}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
