"use client";

import React, { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { ActionMessage, Banner, SectionHeader } from "@/components/documents/shared";
import { PenTool, Save } from "lucide-react";

interface Policy {
  doc_type: string;
  allow_eid: boolean;
  allow_dan: boolean;
  require_named_signer: boolean;
  configured: boolean;
  updated_at?: string;
}

/**
 * Signature policies: which national channel may sign a document type, and
 * whether the signer has to be one the approval chain names. A type nobody has
 * configured accepts both channels and names no signer, which is how the app
 * behaved before this screen existed.
 */
export default function SignaturePoliciesPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");

  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<ActionMessage | null>(null);

  // The server answers with a row for every document type, so an empty list can only
  // mean the load failed. Rendering the table anyway left a blank page that reads as
  // "this tenant has no document types" the moment the error banner is dismissed.
  const [loadFailed, setLoadFailed] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      setPolicies((await api.getSignaturePolicies()) || []);
      setLoadFailed(false);
    } catch (err: any) {
      setLoadFailed(true);
      setMessage({ type: "error", text: err?.message || t("documents.message.policies_failed") });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const edit = (docType: string, patch: Partial<Policy>) =>
    setPolicies((current) => current.map((p) => (p.doc_type === docType ? { ...p, ...patch } : p)));

  const save = async (policy: Policy) => {
    setBusy(policy.doc_type);
    setMessage(null);
    try {
      const saved = (await api.saveSignaturePolicy(policy.doc_type, {
        allow_eid: policy.allow_eid,
        allow_dan: policy.allow_dan,
        require_named_signer: policy.require_named_signer,
      })) as Policy | undefined;
      setMessage({ type: "success", text: t("documents.message.policy_saved", { type: policy.doc_type }) });
      // Only the row that was saved is replaced. Reloading the table reverted the
      // edits an operator had made to the other document types, under a banner
      // naming only this one.
      if (saved && saved.doc_type) edit(policy.doc_type, saved);
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.policies_failed") });
      // This refusal cannot be fixed on this screen — the chain beneath the policy is
      // what has to change — so this row goes back to what is stored. The others keep
      // whatever the operator has typed.
      try {
        const stored = await api.getSignaturePolicies();
        const mine = (stored || []).find((row) => row.doc_type === policy.doc_type);
        if (mine) edit(policy.doc_type, mine);
      } catch {
        // Leave the row as typed rather than blank the screen over a second failure.
      }
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <SectionHeader
        icon={<PenTool className="w-7 h-7 text-indigo-600" />}
        title={t("documents.menu.signature_policies")}
        subtitle={t("documents.view.signature_policies_hint")}
      />

      {message && <Banner message={message} onDismiss={() => setMessage(null)} />}

      {loading ? (
        <div className="py-12 text-center text-slate-400">{t("documents.message.loading")}</div>
      ) : loadFailed ? (
        <div className="bg-white border border-slate-200 rounded-xl p-8 text-center text-slate-500 text-sm">
          {t("documents.message.policies_failed")}
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
          <table className="w-full text-left text-xs text-slate-600">
            <thead className="bg-slate-50 text-slate-700 font-semibold border-b border-slate-200 uppercase">
              <tr>
                <th className="px-4 py-3">{t("base.field.type")}</th>
                <th className="px-4 py-3">E-ID</th>
                <th className="px-4 py-3">DAN</th>
                <th className="px-4 py-3">{t("documents.field.require_named_signer")}</th>
                <th className="px-4 py-3">{t("base.field.status")}</th>
                <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {policies.map((policy) => (
                <tr key={policy.doc_type} className="hover:bg-slate-50">
                  <td className="px-4 py-3 font-mono font-semibold text-slate-900">{policy.doc_type}</td>
                  <td className="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={policy.allow_eid}
                      disabled={!mayManage}
                      onChange={(e) => edit(policy.doc_type, { allow_eid: e.target.checked })}
                      className="w-4 h-4 accent-indigo-600"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={policy.allow_dan}
                      disabled={!mayManage}
                      onChange={(e) => edit(policy.doc_type, { allow_dan: e.target.checked })}
                      className="w-4 h-4 accent-indigo-600"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={policy.require_named_signer}
                      disabled={!mayManage}
                      onChange={(e) => edit(policy.doc_type, { require_named_signer: e.target.checked })}
                      className="w-4 h-4 accent-indigo-600"
                    />
                  </td>
                  <td className="px-4 py-3">
                    {policy.configured ? (
                      <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-indigo-50 text-indigo-700 border border-indigo-200">
                        {t("documents.state.policy_configured")}
                      </span>
                    ) : (
                      <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-slate-100 text-slate-600 border border-slate-200">
                        {t("documents.state.policy_default")}
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {mayManage ? (
                      <button
                        onClick={() => save(policy)}
                        disabled={busy === policy.doc_type || (!policy.allow_eid && !policy.allow_dan)}
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

      <p className="text-xs text-slate-500">{t("documents.message.policy_named_signer_hint")}</p>
    </div>
  );
}
