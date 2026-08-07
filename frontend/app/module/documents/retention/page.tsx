"use client";

import React, { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { ActionMessage, Banner, SectionHeader } from "@/components/documents/shared";
import { Archive, Save } from "lucide-react";

interface Rule {
  doc_type: string;
  retain_years: number;
  note: string;
  configured: boolean;
  updated_at?: string;
  /**
   * Absent when the server could not count — the save path treats a failed count as
   * non-fatal — so the row keeps whatever it already knew rather than claiming the
   * type has nothing filed under it.
   */
  expired?: number;
  total?: number;
}

/**
 * Retention rules: how long each document type is kept. Nothing is deleted on
 * this schedule — the screen reports what is past its term and a person decides,
 * which is the only safe reading of a rule nobody has reviewed yet.
 */
export default function DocumentRetentionPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");

  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<ActionMessage | null>(null);

  // A failed load left the three tiles saying "0 filed · 0 past their term · 0 rules
  // set" — a claim about the tenant, made out of rows the page never received.
  const [loadFailed, setLoadFailed] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      setRules((await api.getRetentionRules()) || []);
      setLoadFailed(false);
    } catch (err: any) {
      setLoadFailed(true);
      setMessage({ type: "error", text: err?.message || t("documents.message.retention_failed") });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const edit = (docType: string, patch: Partial<Rule>) =>
    setRules((current) => current.map((r) => (r.doc_type === docType ? { ...r, ...patch } : r)));

  const save = async (rule: Rule) => {
    setBusy(rule.doc_type);
    setMessage(null);
    try {
      // The saved rule comes back with the counts the server computed, so only this
      // row needs replacing. Reloading the table wiped the years an operator had
      // typed into the other rows — and, on the error path, the very value they were
      // being asked to correct.
      const saved = (await api.saveRetentionRule(rule.doc_type, {
        retain_years: rule.retain_years,
        note: rule.note,
      })) as Rule | undefined;
      setMessage({ type: "success", text: t("documents.message.retention_saved", { type: rule.doc_type }) });
      if (saved && saved.doc_type) {
        // `total` counts every document of the type, so a save cannot change it and
        // the row's own is still good if the server sent none.
        //
        // `expired` is counted AGAINST THE TERM, which is exactly what this save
        // changed — so the row's own belongs to a term that no longer applies, and
        // the direction is always the dangerous one: shortening a term can only
        // increase what is past it, and shortening is the common edit. Left absent,
        // the cell reads "—" instead of stating a stale zero over a type where
        // hundreds are now past their term.
        //
        // `expired` is NAMED in the patch even though it may be undefined: the server
        // omits the key entirely when it could not count, and `edit` merges, so a key
        // that is merely absent leaves the row's old number sitting there. Naming it
        // is what actually clears it.
        edit(rule.doc_type, {
          ...saved,
          expired: saved.expired,
          total: saved.total ?? rule.total,
        });
      }
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.retention_failed") });
    } finally {
      setBusy(null);
    }
  };

  // A tile that adds up rows the server could not count states a total it does not
  // know. It is prefixed with "≥" then, because the number can only be short.
  const expiredTotal = rules.reduce((sum, rule) => sum + (rule.expired ?? 0), 0);
  const filedTotal = rules.reduce((sum, rule) => sum + (rule.total ?? 0), 0);
  const expiredPartial = rules.some((rule) => rule.expired === undefined);
  const filedPartial = rules.some((rule) => rule.total === undefined);
  const atLeast = (value: number, partial: boolean) =>
    loadFailed ? "—" : partial ? `≥${value}` : `${value}`;

  return (
    <div className="space-y-6">
      <SectionHeader
        icon={<Archive className="w-7 h-7 text-indigo-600" />}
        title={t("documents.menu.retention")}
        subtitle={t("documents.view.retention_hint")}
      />

      {message && <Banner message={message} onDismiss={() => setMessage(null)} />}

      <section className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <div className="p-4 bg-white border border-slate-200 rounded-xl">
          <div className="text-2xl font-bold text-slate-700">{atLeast(filedTotal, filedPartial)}</div>
          <div className="text-[11px] text-slate-500 leading-snug mt-1">{t("documents.stat.filed")}</div>
        </div>
        <div className="p-4 bg-white border border-slate-200 rounded-xl">
          <div className={`text-2xl font-bold ${expiredTotal > 0 ? "text-amber-600" : "text-slate-700"}`}>
            {atLeast(expiredTotal, expiredPartial)}
          </div>
          <div className="text-[11px] text-slate-500 leading-snug mt-1">{t("documents.stat.past_term")}</div>
        </div>
        <div className="p-4 bg-white border border-slate-200 rounded-xl">
          <div className="text-2xl font-bold text-indigo-600">
            {loadFailed ? "—" : rules.filter((r) => r.configured).length}
          </div>
          <div className="text-[11px] text-slate-500 leading-snug mt-1">{t("documents.stat.rules_set")}</div>
        </div>
      </section>

      {loading ? (
        <div className="py-12 text-center text-slate-400">{t("documents.message.loading")}</div>
      ) : loadFailed ? (
        // The server answers with a row for every document type, so an empty table
        // could only mean the load failed — and rendering the headers over nothing
        // reads as "this tenant has no document types".
        <div className="bg-white border border-slate-200 rounded-xl p-8 text-center text-slate-500 text-sm">
          {t("documents.message.retention_failed")}
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
          <table className="w-full text-left text-xs text-slate-600">
            <thead className="bg-slate-50 text-slate-700 font-semibold border-b border-slate-200 uppercase">
              <tr>
                <th className="px-4 py-3">{t("base.field.type")}</th>
                <th className="px-4 py-3">{t("documents.field.retain_years")}</th>
                <th className="px-4 py-3">{t("documents.field.retention_note")}</th>
                <th className="px-4 py-3">{t("documents.stat.filed")}</th>
                <th className="px-4 py-3">{t("documents.stat.past_term")}</th>
                <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {rules.map((rule) => (
                <tr key={rule.doc_type} className="hover:bg-slate-50">
                  <td className="px-4 py-3 font-mono font-semibold text-slate-900">{rule.doc_type}</td>
                  <td className="px-4 py-3">
                    <input
                      type="number"
                      min={1}
                      max={100}
                      value={rule.retain_years || ""}
                      disabled={!mayManage}
                      onChange={(e) => edit(rule.doc_type, { retain_years: Number(e.target.value) })}
                      className="w-20 px-2 py-1.5 text-xs border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <input
                      type="text"
                      value={rule.note}
                      disabled={!mayManage}
                      onChange={(e) => edit(rule.doc_type, { note: e.target.value })}
                      className="w-full px-2 py-1.5 text-xs border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                    />
                  </td>
                  <td className="px-4 py-3 text-slate-500">{rule.total ?? "—"}</td>
                  <td className="px-4 py-3">
                    {rule.expired === undefined ? (
                      <span className="text-slate-400">—</span>
                    ) : rule.expired > 0 ? (
                      <span className="text-[11px] font-bold px-2 py-0.5 rounded-full bg-amber-50 text-amber-700 border border-amber-200">
                        {rule.expired}
                      </span>
                    ) : (
                      <span className="text-slate-400">0</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {mayManage ? (
                      <button
                        onClick={() => save(rule)}
                        disabled={busy === rule.doc_type || !rule.retain_years}
                        className="inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold border border-indigo-200 text-indigo-600 hover:bg-indigo-50 disabled:opacity-50"
                      >
                        <Save className="w-3.5 h-3.5" />
                        <span>{t("base.action.save")}</span>
                      </button>
                    ) : (
                      <span className="text-slate-300 text-[11px]">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <p className="text-xs text-slate-500">{t("documents.message.retention_no_deletion")}</p>
    </div>
  );
}
