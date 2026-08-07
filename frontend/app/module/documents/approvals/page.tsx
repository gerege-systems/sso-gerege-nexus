"use client";

/**
 * Approval queue.
 *
 * document_records carries a status, so "waiting" is a real question the data
 * can answer. Acting on a document is not offered here because the documents
 * app exposes no transition endpoint — the screen points, and says where to go.
 */

import { useEffect, useMemo, useState } from "react";
import { CheckCheck, ListChecks } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Panel, Screen, relativeDate } from "@/components/module/kit";

type Document = { id: string; title: string; doc_type: string; status: string; signed_by?: string; created_at: string };

const WAITING = ["DRAFT", "PENDING"];

export default function ApprovalQueuePage() {
  const { t, locale } = useI18n();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setDocuments(((await api.getDocuments()) as Document[]) || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const waiting = useMemo(
    () => documents.filter((d) => WAITING.includes(d.status.toUpperCase()))
      .sort((a, b) => a.created_at.localeCompare(b.created_at)),
    [documents],
  );
  const settled = documents.length - waiting.length;

  return (
    <Screen icon={<ListChecks className="w-5 h-5" />} title={t("mod.documents.approvals.title")} subtitle={t("mod.documents.approvals.subtitle")}>
      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="grid gap-3 sm:grid-cols-2">
        <Panel className="p-4">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{t("mod.documents.approvals.waiting")}</p>
          <p className="text-2xl font-bold text-amber-600 tabular-nums mt-1">{waiting.length}</p>
        </Panel>
        <Panel className="p-4">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{t("mod.documents.approvals.settled")}</p>
          <p className="text-2xl font-bold text-emerald-600 tabular-nums mt-1">{settled}</p>
        </Panel>
      </div>

      <Panel className="p-4 bg-slate-50">
        <p className="text-xs text-slate-600">{t("mod.documents.approvals.note")}</p>
      </Panel>

      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : waiting.length === 0 ? (
        <Empty icon={<CheckCheck className="w-9 h-9 mx-auto" />}>{t("mod.documents.approvals.none")}</Empty>
      ) : (
        <Panel className="divide-y divide-slate-100">
          {waiting.map((document) => (
            <div key={document.id} className="p-4 flex flex-wrap items-center gap-x-3 gap-y-1">
              <div className="min-w-0 flex-1">
                <p className="font-semibold text-slate-900">{document.title}</p>
                <p className="text-[11px] text-slate-400">
                  {document.doc_type}
                  {document.signed_by && ` · ${t("mod.documents.approvals.signed_by")}: ${document.signed_by}`}
                </p>
              </div>
              <Chip tone={document.status.toUpperCase() === "DRAFT" ? "slate" : "amber"} mono>{document.status}</Chip>
              <span className="text-xs text-slate-500 w-28 text-right">
                {relativeDate(document.created_at, "—", locale)}
              </span>
            </div>
          ))}
        </Panel>
      )}
    </Screen>
  );
}
