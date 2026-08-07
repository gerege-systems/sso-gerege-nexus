"use client";

/**
 * Revenue reports.
 *
 * Aggregated in the browser from the invoice list the billing app already
 * serves. That is honest at this size and avoids inventing a reporting endpoint
 * whose numbers could disagree with the list they came from.
 */

import { useEffect, useMemo, useState } from "react";
import { BarChart3, Receipt } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Panel, Screen } from "@/components/module/kit";

type Invoice = {
  id: string; invoice_number: string; contact_name: string;
  amount: number; vat_amount: number; ebarimt_status: string; status: string; created_at: string;
};

const money = (value: number) => new Intl.NumberFormat("mn-MN", { maximumFractionDigits: 0 }).format(value) + "₮";

export default function RevenueReportsPage() {
  const { t } = useI18n();
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setInvoices((await api.getInvoices()) || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const totals = useMemo(() => {
    const sum = (list: Invoice[], field: "amount" | "vat_amount") =>
      list.reduce((total, invoice) => total + Number(invoice[field] || 0), 0);

    const group = (field: "status" | "ebarimt_status") => {
      const buckets = new Map<string, { count: number; amount: number }>();
      for (const invoice of invoices) {
        const key = invoice[field] || "—";
        const current = buckets.get(key) || { count: 0, amount: 0 };
        buckets.set(key, { count: current.count + 1, amount: current.amount + Number(invoice.amount || 0) });
      }
      return [...buckets.entries()].sort((a, b) => b[1].amount - a[1].amount);
    };

    const months = new Map<string, number>();
    for (const invoice of invoices) {
      const month = invoice.created_at.slice(0, 7);
      months.set(month, (months.get(month) || 0) + Number(invoice.amount || 0));
    }

    return {
      amount: sum(invoices, "amount"),
      vat: sum(invoices, "vat_amount"),
      byStatus: group("status"),
      byEbarimt: group("ebarimt_status"),
      byMonth: [...months.entries()].sort((a, b) => a[0].localeCompare(b[0])),
    };
  }, [invoices]);

  const peak = Math.max(1, ...totals.byMonth.map(([, amount]) => amount));

  return (
    <Screen icon={<BarChart3 className="w-5 h-5" />} title={t("mod.billing.reports.title")} subtitle={t("mod.billing.reports.subtitle")}>
      {error && <ErrorNote>{error}</ErrorNote>}

      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : invoices.length === 0 ? (
        <Empty icon={<Receipt className="w-9 h-9 mx-auto" />}>{t("mod.billing.reports.none")}</Empty>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-3">
            <Panel className="p-4">
              <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{t("mod.billing.reports.total")}</p>
              <p className="text-2xl font-bold text-slate-900 tabular-nums mt-1">{money(totals.amount)}</p>
            </Panel>
            <Panel className="p-4">
              <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{t("mod.billing.reports.vat")}</p>
              <p className="text-2xl font-bold text-slate-900 tabular-nums mt-1">{money(totals.vat)}</p>
            </Panel>
            <Panel className="p-4">
              <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{t("mod.billing.reports.count")}</p>
              <p className="text-2xl font-bold text-slate-900 tabular-nums mt-1">{invoices.length}</p>
            </Panel>
          </div>

          <Panel className="p-5">
            <h2 className="text-sm font-bold text-slate-900 mb-3">{t("mod.billing.reports.by_month")}</h2>
            <ul className="space-y-2">
              {totals.byMonth.map(([month, amount]) => (
                <li key={month} className="flex items-center gap-3 text-sm">
                  <span className="w-20 font-mono text-xs text-slate-500">{month}</span>
                  <span className="flex-1 h-2 bg-slate-100 rounded-full overflow-hidden">
                    <span className="block h-full bg-indigo-500" style={{ width: `${(amount / peak) * 100}%` }} />
                  </span>
                  <span className="w-32 text-right tabular-nums text-slate-900">{money(amount)}</span>
                </li>
              ))}
            </ul>
          </Panel>

          <div className="grid gap-3 md:grid-cols-2">
            {([[t("mod.billing.reports.by_status"), totals.byStatus], [t("mod.billing.reports.by_ebarimt"), totals.byEbarimt]] as const).map(([title, rows]) => (
              <Panel key={title} className="p-5">
                <h2 className="text-sm font-bold text-slate-900 mb-3">{title}</h2>
                <ul className="space-y-2">
                  {rows.map(([label, bucket]) => (
                    <li key={label} className="flex items-center gap-2 text-sm">
                      <Chip mono>{label}</Chip>
                      <span className="text-xs text-slate-400">{bucket.count}</span>
                      <span className="ml-auto tabular-nums text-slate-900">{money(bucket.amount)}</span>
                    </li>
                  ))}
                </ul>
              </Panel>
            ))}
          </div>
        </>
      )}
    </Screen>
  );
}
