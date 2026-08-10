"use client";

import React from "react";
import { Dashboard, GovApiError, TaskStatus } from "@/lib/gov";
import { useI18n } from "@/lib/i18n";

const STATUS_STYLE: Record<TaskStatus, string> = {
  RECEIVED: "bg-slate-100 text-slate-700",
  ASSIGNED: "bg-sky-100 text-sky-700",
  IN_PROGRESS: "bg-blue-100 text-blue-700",
  FORWARDED: "bg-violet-100 text-violet-700",
  INFO_REQUESTED: "bg-amber-100 text-amber-700",
  AWAITING_VERIFICATION: "bg-orange-100 text-orange-700",
  RETURNED: "bg-rose-100 text-rose-700",
  COMPLETED: "bg-emerald-100 text-emerald-700",
  CLOSED: "bg-indigo-100 text-indigo-700",
  REJECTED: "bg-red-100 text-red-700",
  CANCELLED: "bg-slate-100 text-slate-500",
};

export const ALL_STATUSES = Object.keys(STATUS_STYLE) as TaskStatus[];

/** Status label and badge, translated through the shared dictionary. */
export function useStatusLabel() {
  const { t } = useI18n();
  return (status: TaskStatus) => {
    // The API sends the state in upper case; the dictionary holds it in the
    // lower-case technical form Odoo uses for selection values.
    const key = `gov.state.${status.toLowerCase()}`;
    const label = t(key as never);
    return label === key ? status : label;
  };
}

export function StatusBadge({ status }: { status: TaskStatus }) {
  const label = useStatusLabel();
  return (
    <span className={`text-xs font-semibold px-2 py-0.5 rounded ${STATUS_STYLE[status] || "bg-slate-100"}`}>
      {label(status)}
    </span>
  );
}

/** Turns any thrown value into a message carrying the backend's machine code. */
export function describeError(err: unknown, fallback: string): string {
  if (err instanceof GovApiError) return `${err.message} (${err.code})`;
  return fallback;
}

/** The counters every level uses to monitor its authorised scope. */
export function DashboardCards({ dashboard }: { dashboard: Dashboard | null }) {
  const { t } = useI18n();
  if (!dashboard) return null;

  const cards = [
    { key: "received", label: t("gov.stat.received"), value: dashboard.received, tone: "text-slate-700" },
    { key: "in_progress", label: t("gov.stat.in_progress"), value: dashboard.in_progress, tone: "text-blue-600" },
    { key: "delegated", label: t("gov.stat.delegated"), value: dashboard.delegated, tone: "text-violet-600" },
    {
      key: "verify",
      label: t("gov.stat.awaiting_verification"),
      value: dashboard.awaiting_verification,
      tone: "text-orange-600",
    },
    { key: "returned", label: t("gov.stat.returned"), value: dashboard.returned, tone: "text-rose-600" },
    {
      key: "completed",
      label: t("gov.stat.completed"),
      value: dashboard.completed + dashboard.closed,
      tone: "text-emerald-600",
    },
    { key: "overdue", label: t("gov.stat.overdue"), value: dashboard.overdue, tone: "text-red-600" },
  ];

  return (
    <section className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-7 gap-3">
      {cards.map((card) => (
        <div key={card.key} className="p-4 bg-white border border-slate-200 rounded-xl">
          <div className={`text-2xl font-bold ${card.tone}`}>{card.value}</div>
          <div className="text-[11px] text-slate-500 leading-snug mt-1">{card.label}</div>
        </div>
      ))}
    </section>
  );
}
