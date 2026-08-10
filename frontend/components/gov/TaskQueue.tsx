"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  ACTION_PERMISSION,
  ACTION_REQUIRES_COMMENT,
  Dashboard,
  OrgUnit,
  Page,
  RequestDetail,
  Task,
  TaskAction,
  availableActions,
  gov,
} from "@/lib/gov";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import RequestDrawer from "@/components/gov/RequestDrawer";
import { Banner, EmptyState, Loading } from "@/components/ui";
import { ALL_STATUSES, DashboardCards, StatusBadge, describeError, useStatusLabel } from "@/components/gov/shared";
import { Loader2 } from "lucide-react";

const PAGE_SIZE = 20;

/**
 * The operational queue: counters, filters, the task table with context-valid
 * actions, and the request drawer. Shared by the requests screen and the app
 * overview so both stay in step.
 */
export default function TaskQueue({ showDashboard = true }: { showDashboard?: boolean }) {
  const { t, locale } = useI18n();
  const statusLabel = useStatusLabel();
  const access = useAccess();

  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [queue, setQueue] = useState<Page<Task> | null>(null);
  const [units, setUnits] = useState<OrgUnit[]>([]);
  const [detail, setDetail] = useState<RequestDetail | null>(null);

  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [statusFilter, setStatusFilter] = useState("");
  const [unitFilter, setUnitFilter] = useState("");
  const [overdueOnly, setOverdueOnly] = useState(false);
  const [page, setPage] = useState(1);

  const report = useCallback((err: unknown) => setError(describeError(err, t("base.message.error"))), [t]);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [summary, tasks] = await Promise.all([
        // The counters need gov.report, which the queue itself does not. A
        // member who may work the queue but not see supervisory totals still
        // gets the table.
        gov.dashboard().catch(() => null),
        gov.tasks({
          status: statusFilter || undefined,
          unit_id: unitFilter || undefined,
          overdue: overdueOnly || undefined,
          page,
          page_size: PAGE_SIZE,
        }),
      ]);
      setDashboard(summary);
      setQueue(tasks);
    } catch (err) {
      report(err);
    }
  }, [statusFilter, unitFilter, overdueOnly, page, report]);

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        setUnits(await gov.units());
      } catch (err) {
        report(err);
      }
      await load();
      setLoading(false);
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!loading) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, unitFilter, overdueOnly, page]);

  const unitName = useMemo(() => {
    const map = new Map(units.map((u) => [u.id, u.name]));
    return (id?: string | null) => (id ? map.get(id) || id : "—");
  }, [units]);

  const act = async (task: Task, action: TaskAction) => {
    let comment = "";
    if (ACTION_REQUIRES_COMMENT.includes(action)) {
      comment = window.prompt(t("gov.message.comment_prompt")) || "";
      if (!comment) return;
    }

    let targetUnit: string | undefined;
    if (action === "delegate") {
      const children = units.filter((u) => u.parent_id === task.unit_id && u.active);
      if (children.length === 0) {
        setError(t("gov.message.no_child_unit"));
        return;
      }
      targetUnit =
        children.length === 1
          ? children[0].id
          : window.prompt(`${t("gov.message.delegate_to")}\n${children.map((c) => `${c.code} = ${c.id}`).join("\n")}`) || "";
      if (!targetUnit) return;
    }

    setBusy(task.id);
    setError(null);
    try {
      const updated = await gov.act(task.id, {
        action,
        row_version: task.row_version,
        comment: comment || undefined,
        target_unit_id: targetUnit,
      });
      setNotice(`${task.reference || ""} → ${statusLabel(updated.status)}`);
      await load();
      if (detail?.request.id === task.application_id) {
        setDetail(await gov.requestDetail(task.application_id));
      }
    } catch (err) {
      report(err);
    } finally {
      setBusy(null);
    }
  };

  const openDetail = async (applicationID: string) => {
    setError(null);
    try {
      setDetail(await gov.requestDetail(applicationID));
    } catch (err) {
      report(err);
    }
  };

  if (loading || !access.ready) return <Loading />;

  return (
    <div className="space-y-6">
      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}
      {notice && <Banner tone="success" message={notice} onDismiss={() => setNotice(null)} />}

      {showDashboard && <DashboardCards dashboard={dashboard} />}

      <div className="flex flex-wrap gap-2 items-center">
        <select
          value={statusFilter}
          onChange={(e) => {
            setPage(1);
            setStatusFilter(e.target.value);
          }}
          className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white"
        >
          <option value="">{t("gov.filter.all_statuses")}</option>
          {ALL_STATUSES.map((status) => (
            <option key={status} value={status}>
              {statusLabel(status)}
            </option>
          ))}
        </select>

        <select
          value={unitFilter}
          onChange={(e) => {
            setPage(1);
            setUnitFilter(e.target.value);
          }}
          className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white"
        >
          <option value="">{t("gov.filter.all_units")}</option>
          {units.map((unit) => (
            <option key={unit.id} value={unit.id}>
              {locale === "en" && unit.name_en ? unit.name_en : unit.name}
            </option>
          ))}
        </select>

        <label className="flex items-center gap-2 text-sm text-slate-600">
          <input
            type="checkbox"
            checked={overdueOnly}
            onChange={(e) => {
              setPage(1);
              setOverdueOnly(e.target.checked);
            }}
          />
          {t("gov.filter.overdue_only")}
        </label>
      </div>

      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
        {!queue || queue.items.length === 0 ? (
          <EmptyState message={t("gov.message.queue_empty")} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[880px]">
              <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                <tr>
                  <th className="px-4 py-3">{t("gov.field.reference")}</th>
                  <th className="px-4 py-3">{t("gov.field.service")}</th>
                  <th className="px-4 py-3">{t("gov.field.unit")}</th>
                  <th className="px-4 py-3">{t("base.field.status")}</th>
                  <th className="px-4 py-3">{t("gov.field.due")}</th>
                  <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {queue.items.map((task) => (
                  <tr key={task.id} className={task.overdue ? "bg-red-50/40" : undefined}>
                    <td className="px-4 py-3">
                      <button
                        onClick={() => openDetail(task.application_id)}
                        className="font-mono text-xs text-[var(--gerege-blue)] hover:underline"
                      >
                        {task.reference}
                      </button>
                    </td>
                    <td className="px-4 py-3">{task.service_name}</td>
                    <td className="px-4 py-3">{task.unit_name || unitName(task.unit_id)}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={task.status} />
                    </td>
                    <td className="px-4 py-3 text-xs">
                      {task.due_at ? (
                        <span className={task.overdue ? "text-red-600 font-semibold" : "text-slate-500"}>
                          {new Date(task.due_at).toLocaleDateString(locale === "en" ? "en-GB" : "mn-MN")}
                          {task.overdue ? ` · ${t("gov.label.overdue")}` : ""}
                        </span>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1.5 justify-end">
                        {busy === task.id ? (
                          <Loader2 className="w-4 h-4 animate-spin text-slate-400" />
                        ) : (
                          availableActions(task.status)
                            .filter((action) => access.can(ACTION_PERMISSION[action]))
                            .map((action) => (
                              <button
                                key={action}
                                onClick={() => act(task, action)}
                                className="text-xs px-2 py-1 rounded border border-slate-200 hover:bg-slate-50 text-slate-600"
                              >
                                {t(`gov.action.${action}` as never)}
                              </button>
                            ))
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {queue && queue.total > PAGE_SIZE && (
        <div className="flex items-center justify-between text-sm text-slate-500">
          <span>
            {t("base.message.page_of")
              .replace("{page}", String(queue.page))
              .replace("{total}", String(Math.ceil(queue.total / queue.page_size)))}
          </span>
          <div className="flex gap-2">
            <button
              disabled={queue.page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              className="px-3 py-1.5 border border-slate-200 rounded-lg disabled:opacity-40"
            >
              {t("base.action.previous")}
            </button>
            <button
              disabled={queue.page * queue.page_size >= queue.total}
              onClick={() => setPage((p) => p + 1)}
              className="px-3 py-1.5 border border-slate-200 rounded-lg disabled:opacity-40"
            >
              {t("base.action.next")}
            </button>
          </div>
        </div>
      )}

      {detail && (
        <RequestDrawer
          detail={detail}
          onClose={() => setDetail(null)}
          unitName={unitName}
          statusBadge={(status) => <StatusBadge status={status} />}
        />
      )}
    </div>
  );
}
