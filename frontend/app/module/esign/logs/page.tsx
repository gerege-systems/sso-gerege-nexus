"use client";

/**
 * Signature logs.
 *
 * esign_signature_logs has been written on every certificate check and signing
 * since the e-sign app shipped, and GET /esign/logs has been serving it — with
 * nothing reading either. This is the audit trail those rows were for.
 */

import { useEffect, useMemo, useState } from "react";
import { FileSignature, ScrollText } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Panel, Screen } from "@/components/module/kit";

type Log = {
  id: string; document_id: string; reg_no: string; phone_no: string;
  first_name: string; last_name: string; action: string; created_at: string;
};

/** mask keeps enough of an identifier to recognise, not enough to reuse. */
function mask(value: string, keep = 2) {
  const trimmed = (value || "").trim();
  if (trimmed.length <= keep) return trimmed || "—";
  return trimmed.slice(0, keep) + "•".repeat(Math.max(3, trimmed.length - keep - 2)) + trimmed.slice(-2);
}

export default function SignatureLogsPage() {
  const { t } = useI18n();
  const [logs, setLogs] = useState<Log[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setLogs(((await api.getEsignLogs()) as Log[]) || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const actions = useMemo(() => [...new Set(logs.map((l) => l.action))].sort(), [logs]);
  const shown = useMemo(() => (filter ? logs.filter((l) => l.action === filter) : logs), [logs, filter]);

  return (
    <Screen
      icon={<ScrollText className="w-5 h-5" />}
      title={t("mod.esign.logs.title")}
      subtitle={t("mod.esign.logs.subtitle")}
      action={
        actions.length > 1 && (
          <div className="flex flex-wrap gap-1">
            <button
              onClick={() => setFilter("")}
              className={`text-[11px] font-mono px-2.5 py-1 rounded-lg border ${filter === "" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-slate-200 text-slate-500"}`}
            >
              all
            </button>
            {actions.map((action) => (
              <button
                key={action}
                onClick={() => setFilter(action)}
                className={`text-[11px] font-mono px-2.5 py-1 rounded-lg border ${filter === action ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-slate-200 text-slate-500"}`}
              >
                {action}
              </button>
            ))}
          </div>
        )
      }
    >
      {error && <ErrorNote>{error}</ErrorNote>}
      <Panel className="p-4 bg-slate-50">
        <p className="text-xs text-slate-600">{t("mod.esign.logs.masked_note")}</p>
      </Panel>

      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : shown.length === 0 ? (
        <Empty icon={<FileSignature className="w-9 h-9 mx-auto" />}>{t("mod.esign.logs.none")}</Empty>
      ) : (
        <Panel className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 border-b border-slate-200 text-left text-[11px] uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-2.5 font-semibold">{t("mod.esign.logs.when")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("mod.esign.logs.action")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("mod.esign.logs.signer")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("mod.esign.logs.document")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {shown.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-50/60">
                    <td className="px-4 py-3 text-slate-600 whitespace-nowrap">
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-3">
                      <Chip mono tone={log.action === "SIGN" ? "emerald" : "slate"}>{log.action}</Chip>
                    </td>
                    <td className="px-4 py-3">
                      <span className="font-medium text-slate-900">
                        {[log.last_name, log.first_name].filter(Boolean).join(" ") || "—"}
                      </span>
                      <span className="block text-[11px] font-mono text-slate-400">
                        {mask(log.reg_no)} · {mask(log.phone_no)}
                      </span>
                    </td>
                    <td className="px-4 py-3 font-mono text-[11px] text-slate-400">
                      {log.document_id ? log.document_id.slice(0, 8) : "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
    </Screen>
  );
}
