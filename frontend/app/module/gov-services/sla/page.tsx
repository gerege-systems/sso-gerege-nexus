"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Dashboard, Page, RequestDetail, Task, Workflow, WorkflowVersion, gov } from "@/lib/gov";
import { useI18n } from "@/lib/i18n";
import RequestDrawer from "@/components/gov/RequestDrawer";
import { Banner, EmptyState, Loading, PageHeader } from "@/components/ui";
import { DashboardCards, StatusBadge, describeError } from "@/components/gov/shared";
import { Timer } from "lucide-react";

/** Service levels: the time standard each workflow step carries, and the work
 *  that has run past it. Overdue is derived from due_at, so nothing here
 *  rewrites a business status. */
export default function GovSlaPage() {
  const { t, locale } = useI18n();
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [versions, setVersions] = useState<{ workflow: Workflow; version: WorkflowVersion }[]>([]);
  const [overdue, setOverdue] = useState<Page<Task> | null>(null);
  const [detail, setDetail] = useState<RequestDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const report = useCallback((err: unknown) => setError(describeError(err, t("base.message.error"))), [t]);

  const load = useCallback(async () => {
    try {
      const [summary, workflows, overdueTasks] = await Promise.all([
        // Counters need gov.report; the step standards and the overdue list do
        // not, so a missing grant hides the cards instead of failing the page.
        gov.dashboard().catch(() => null),
        gov.workflows(),
        gov.tasks({ overdue: true, page_size: 50 }),
      ]);
      setDashboard(summary);
      setOverdue(overdueTasks);

      // Step SLAs live on the published version, so each one is fetched by id.
      const published = workflows.flatMap((wf) =>
        (wf.versions || []).filter((v) => v.status === "PUBLISHED").map((v) => ({ workflow: wf, version: v })),
      );
      const detailed = await Promise.all(
        published.map(async (entry) => ({
          workflow: entry.workflow,
          version: await gov.workflowVersion(entry.version.id),
        })),
      );
      setVersions(detailed);
    } catch (err) {
      report(err);
    }
  }, [report]);

  useEffect(() => {
    (async () => {
      setLoading(true);
      await load();
      setLoading(false);
    })();
  }, [load]);

  if (loading) return <Loading />;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Timer className="w-7 h-7 text-[var(--gerege-blue)]" />}
        title={t("gov.menu.sla")}
        subtitle={t("gov.view.sla_hint")}
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      <DashboardCards dashboard={dashboard} />

      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-4">
        <h2 className="font-semibold">{t("gov.view.workflows")}</h2>
        {versions.length === 0 ? (
          <p className="text-sm text-slate-500">{t("gov.message.no_published_workflow")}</p>
        ) : (
          versions.map(({ workflow, version }) => (
            <div key={version.id} className="space-y-2">
              <h3 className="text-sm font-medium text-slate-700">
                {locale === "en" && workflow.name_en ? workflow.name_en : workflow.name}
                <span className="ml-2 text-xs text-slate-400">v{version.version}</span>
              </h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm min-w-[560px]">
                  <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                    <tr>
                      <th className="px-3 py-2">{t("gov.field.step")}</th>
                      <th className="px-3 py-2">{t("gov.field.strategy")}</th>
                      <th className="px-3 py-2">{t("gov.field.sla_hours")}</th>
                      <th className="px-3 py-2">{t("gov.field.requires_verification")}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {(version.steps || []).map((step) => (
                      <tr key={step.id}>
                        <td className="px-3 py-2">
                          <span className="font-mono text-xs text-slate-400 mr-2">{step.step_order}</span>
                          {step.name}
                        </td>
                        <td className="px-3 py-2 font-mono text-xs">{step.executor_rule}</td>
                        <td className="px-3 py-2">{step.sla_hours}</td>
                        <td className="px-3 py-2">
                          {step.requires_verification ? (
                            <span className="text-xs font-semibold px-2 py-0.5 rounded bg-orange-100 text-orange-700">
                              {t("base.value.yes")}
                            </span>
                          ) : (
                            <span className="text-xs text-slate-400">{t("base.value.no")}</span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ))
        )}
      </section>

      <section className="space-y-2">
        <h2 className="font-semibold">{t("gov.view.overdue_tasks")}</h2>
        <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
          {!overdue || overdue.items.length === 0 ? (
            <EmptyState message={t("gov.message.queue_empty")} />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm min-w-[720px]">
                <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                  <tr>
                    <th className="px-4 py-3">{t("gov.field.reference")}</th>
                    <th className="px-4 py-3">{t("gov.field.service")}</th>
                    <th className="px-4 py-3">{t("gov.field.unit")}</th>
                    <th className="px-4 py-3">{t("base.field.status")}</th>
                    <th className="px-4 py-3">{t("gov.field.due")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {overdue.items.map((task) => (
                    <tr key={task.id} className="bg-red-50/40">
                      <td className="px-4 py-3">
                        <button
                          onClick={async () => {
                            try {
                              setDetail(await gov.requestDetail(task.application_id));
                            } catch (err) {
                              report(err);
                            }
                          }}
                          className="font-mono text-xs text-[var(--gerege-blue)] hover:underline"
                        >
                          {task.reference}
                        </button>
                      </td>
                      <td className="px-4 py-3">{task.service_name}</td>
                      <td className="px-4 py-3">{task.unit_name}</td>
                      <td className="px-4 py-3">
                        <StatusBadge status={task.status} />
                      </td>
                      <td className="px-4 py-3 text-xs text-red-600 font-semibold">
                        {task.due_at ? new Date(task.due_at).toLocaleString(locale === "en" ? "en-GB" : "mn-MN") : "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </section>

      {detail && (
        <RequestDrawer
          detail={detail}
          onClose={() => setDetail(null)}
          unitName={(id) => id || "—"}
          statusBadge={(status) => <StatusBadge status={status} />}
        />
      )}
    </div>
  );
}
