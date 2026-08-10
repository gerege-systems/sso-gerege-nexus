"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useI18n } from "@/lib/i18n";
import { Modal, fieldClass } from "@/components/ui";
import { Users, Plus, Mail, CheckCircle, XCircle } from "lucide-react";

interface Contact {
  id: string;
  name: string;
  email: string;
  phone: string;
  company: string;
  active: boolean;
}

export default function ContactsPage() {
  const { t } = useI18n();
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({ name: "", email: "", phone: "", company: "", active: true });
  const [error, setError] = useState("");

  const { data: contacts, loading, reload: loadContacts } = useResource(
    async () => (await api.getContacts()) || [],
    {
      initial: [] as Contact[],
      onError: (err: any) => setError(err.message || t("contacts.message.load_failed")),
    },
  );

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.createContact(form);
      setShowModal(false);
      setForm({ name: "", email: "", phone: "", company: "", active: true });
      await loadContacts();
    } catch (err: any) {
      setError(err.message || t("contacts.message.create_failed"));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-slate-200 pb-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
            <Users className="w-6 h-6 text-indigo-600" />
            <span>{t("contacts.view.title")}</span>
          </h1>
          <p className="text-sm text-slate-500">{t("contacts.view.subtitle")}</p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-sm py-2 px-4 rounded-lg flex items-center space-x-2 transition"
        >
          <Plus className="w-4 h-4" />
          <span>{t("contacts.action.create")}</span>
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg">
          {error}
        </div>
      )}

      {loading ? (
        <div className="py-8 text-slate-500 text-sm">{t("contacts.message.loading")}</div>
      ) : contacts.length === 0 ? (
        <div className="bg-white border border-slate-200 rounded-xl p-12 text-center text-slate-500 text-sm">
          {t("contacts.message.empty")}
        </div>
      ) : (
        <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse text-sm">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-200 text-slate-600 font-semibold text-xs uppercase tracking-wider">
                <th className="py-3 px-4">{t("base.field.name")}</th>
                <th className="py-3 px-4">{t("base.field.email")}</th>
                <th className="py-3 px-4">{t("base.field.phone")}</th>
                <th className="py-3 px-4">{t("base.field.company")}</th>
                <th className="py-3 px-4">{t("base.field.status")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {contacts.map((c) => (
                <tr key={c.id} className="hover:bg-slate-50">
                  <td className="py-3.5 px-4 font-bold text-slate-900">{c.name}</td>
                  <td className="py-3.5 px-4 text-slate-600 flex items-center space-x-1 pt-4">
                    <Mail className="w-3.5 h-3.5 text-slate-400" />
                    <span>{c.email || "—"}</span>
                  </td>
                  <td className="py-3.5 px-4 text-slate-600">{c.phone || "—"}</td>
                  <td className="py-3.5 px-4 text-slate-700 font-medium">{c.company || "—"}</td>
                  <td className="py-3.5 px-4">
                    <span
                      className={`inline-flex items-center space-x-1 px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                        c.active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-600"
                      }`}
                    >
                      {c.active ? <CheckCircle className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                      <span>{c.active ? "Active" : "Inactive"}</span>
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
        <Modal label={t("contacts.view.create_title")}>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-slate-900">{t("contacts.view.create_title")}</h2>
            <button
              type="button"
              onClick={async () => {
                try {
                  const info = await api.queryXYPCitizen("AA90010111");
                  setForm({
                    name: `${info.last_name} ${info.first_name}`,
                    email: `${info.reg_number.toLowerCase()}@gerege.mn`,
                    phone: "99112233",
                    company: t("contacts.message.xyp_verified"),
                    active: true,
                  });
                } catch (err: any) {
                  alert(t("contacts.message.xyp_failed") + ": " + err.message);
                }
              }}
              className="bg-blue-50 hover:bg-blue-100 text-blue-700 text-xs font-semibold px-2.5 py-1.5 rounded-lg border border-blue-200 transition"
            >{t("contacts.action.xyp_autofill")}</button>
          </div>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("contacts.field.full_name")} *</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                className={fieldClass}
                required
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("base.field.email")}</label>
              <input
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                className={fieldClass}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("base.field.phone")}</label>
              <input
                type="text"
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
                className={fieldClass}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">{t("base.field.company")}</label>
              <input
                type="text"
                value={form.company}
                onChange={(e) => setForm({ ...form, company: e.target.value })}
                className={fieldClass}
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
                className="w-1/2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium py-2 rounded-lg text-sm"
              >
                {t("contacts.action.save")}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
