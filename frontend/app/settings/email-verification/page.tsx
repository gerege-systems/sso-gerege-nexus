"use client";

import React, { useCallback, useEffect, useState } from "react";
import { EmailVerification, EmailVerifyOverview, api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, fieldClass } from "@/components/ui";
import {
  CheckCircle2, Clock, ExternalLink, Info, KeyRound, MailCheck,
  RefreshCw, Send, XCircle,
} from "lucide-react";

/**
 * Email verification lives in Settings, not in an app.
 *
 * Proving an address is not one module's business — Contacts, Documents and Gov
 * Services all want it — so the screen administers the shared capability rather
 * than being a feature of whichever app asked first.
 *
 * There is no key management on it. The mail is sent by the hosted service, the
 * key is that service's, and it is administered there; this platform's copy of
 * it is a server-side environment variable that deliberately never reaches this
 * page. So what an administrator gets here is the two questions they actually
 * have: is the service working, and who has been written to.
 */

export default function EmailVerificationPage() {
  const { t } = useI18n();
  const [overview, setOverview] = useState<EmailVerifyOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [banner, setBanner] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [testEmail, setTestEmail] = useState("");
  const [testRedirect, setTestRedirect] = useState("");

  const report = useCallback((err: unknown, fallback: string) => {
    const message = err instanceof Error && err.message ? err.message : fallback;
    setBanner({ kind: "error", text: message });
  }, []);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      setOverview(await api.getEmailVerifyOverview());
    } catch (err) {
      report(err, t("emailverify.message.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [report, t]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  async function handleTestSend(e: React.FormEvent) {
    e.preventDefault();
    setBanner(null);
    setBusy(true);
    try {
      await api.sendEmailVerification({
        email: testEmail,
        redirect_url: testRedirect || undefined,
        purpose: "portal_test",
      });
      setBanner({ kind: "ok", text: t("emailverify.message.test_sent", { email: testEmail }) });
      setTestEmail("");
      await loadData();
    } catch (err) {
      report(err, t("emailverify.message.send_failed"));
    } finally {
      setBusy(false);
    }
  }

  const stats = overview?.stats;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
            <MailCheck className="w-7 h-7 text-indigo-600" />
            <span>{t("emailverify.view.title")}</span>
          </h1>
          <p className="text-sm text-slate-500 mt-1">{t("emailverify.view.subtitle")}</p>
        </div>
        <button
          onClick={() => void loadData()}
          aria-label={t("base.action.retry")}
          className="p-2 text-slate-600 hover:bg-slate-100 rounded-lg border border-slate-200 transition"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      {banner && (
        <Banner tone={banner.kind === "ok" ? "success" : "error"} message={banner.text} />
      )}

      {/* A missing key and an unreachable service look identical from the
          outside — nothing arrives — so they are told apart here. */}
      {overview && !overview.configured && (
        <Banner tone="warning" message={t("emailverify.message.not_configured")} />
      )}

      {loading ? (
        <LoadingBlock label={t("emailverify.message.loading")} />
      ) : (
        <>
          <section className="bg-white border border-slate-200 rounded-xl p-5">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <h2 className="font-bold text-slate-900 text-sm flex items-center gap-2">
                  <KeyRound className="w-4 h-4 text-indigo-600" />
                  {t("emailverify.view.service_title")}
                </h2>
                <dl className="mt-3 space-y-1.5 text-xs">
                  <div className="flex gap-2">
                    <dt className="text-slate-500 w-40 shrink-0">{t("emailverify.field.provider")}</dt>
                    <dd className="font-mono text-slate-700 truncate">{overview?.provider_url}</dd>
                  </div>
                  <div className="flex gap-2">
                    <dt className="text-slate-500 w-40 shrink-0">{t("emailverify.field.return_url")}</dt>
                    <dd className="font-mono text-slate-700 truncate">{overview?.return_url}</dd>
                  </div>
                </dl>
              </div>
              <div className="flex flex-col items-end gap-2 shrink-0">
                <span
                  className={`inline-flex items-center gap-1.5 text-xs font-bold px-2.5 py-1 rounded-full border ${
                    overview?.reachable
                      ? "bg-emerald-50 text-emerald-700 border-emerald-200"
                      : "bg-red-50 text-red-700 border-red-200"
                  }`}
                >
                  {overview?.reachable ? (
                    <CheckCircle2 className="w-3.5 h-3.5" />
                  ) : (
                    <XCircle className="w-3.5 h-3.5" />
                  )}
                  {overview?.reachable
                    ? t("emailverify.message.reachable")
                    : t("emailverify.message.unreachable", { reason: overview?.health || "—" })}
                </span>
                {overview?.admin_url && (
                  <a
                    href={overview.admin_url}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="inline-flex items-center gap-1.5 text-xs font-semibold text-indigo-600 hover:text-indigo-700"
                  >
                    {t("emailverify.action.open_admin")}
                    <ExternalLink className="w-3.5 h-3.5" />
                  </a>
                )}
              </div>
            </div>
          </section>

          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            <StatCard label={t("emailverify.stat.total")} value={stats?.total ?? 0} />
            <StatCard label={t("emailverify.stat.verified")} value={stats?.verified ?? 0} tone="ok" />
            <StatCard label={t("emailverify.stat.pending")} value={stats?.pending ?? 0} tone="wait" />
            <StatCard label={t("emailverify.stat.expired")} value={stats?.expired ?? 0} tone="off" />
            <StatCard
              label={t("emailverify.stat.verified_pct")}
              value={`${Math.round(stats?.verified_pct ?? 0)}%`}
            />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-3">
              <h2 className="font-bold text-slate-900 text-sm">{t("emailverify.view.usage_title")}</h2>
              <p className="text-xs text-slate-500">{t("emailverify.message.usage")}</p>
              <p className="text-xs text-slate-500">{t("emailverify.message.in_app_usage")}</p>
              {/* Pending does not mean "they ignored it" — say so where the
                  word is being read, not only in the code. */}
              <p className="text-xs text-amber-800 bg-amber-50 border border-amber-200 rounded-lg p-2.5 flex items-start gap-2">
                <Info className="w-3.5 h-3.5 mt-0.5 shrink-0" />
                <span>{t("emailverify.message.no_webhook_note")}</span>
              </p>
            </section>

            <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-3">
              <h2 className="font-bold text-slate-900 text-sm">{t("emailverify.view.test_title")}</h2>
              <form onSubmit={handleTestSend} className="space-y-3">
                <input
                  type="email"
                  required
                  value={testEmail}
                  onChange={(e) => setTestEmail(e.target.value)}
                  placeholder="user@example.com"
                  className={fieldClass}
                />
                <input
                  type="url"
                  value={testRedirect}
                  onChange={(e) => setTestRedirect(e.target.value)}
                  placeholder="https://theirapp.com/verified"
                  className={fieldClass}
                />
                <button
                  type="submit"
                  disabled={busy || !overview?.configured}
                  className="w-full flex items-center justify-center gap-2 bg-slate-900 hover:bg-slate-800 text-white text-xs font-semibold py-2 rounded-lg disabled:opacity-50"
                >
                  <Send className="w-3.5 h-3.5" />
                  {t("emailverify.action.send_test")}
                </button>
              </form>
            </section>
          </div>

          <section className="bg-white border border-slate-200 rounded-xl overflow-hidden">
            <header className="px-5 py-3 border-b border-slate-200">
              <h2 className="font-bold text-slate-900 text-sm flex items-center gap-2">
                <Clock className="w-4 h-4 text-indigo-600" />
                {t("emailverify.view.recent_title")}
              </h2>
            </header>
            {(overview?.recent.length ?? 0) === 0 ? (
              <p className="p-8 text-center text-sm text-slate-500">
                {t("emailverify.message.no_verifications")}
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-slate-50 text-[11px] uppercase tracking-wide text-slate-500">
                    <tr>
                      <th className="text-left font-semibold px-5 py-2">{t("base.field.email")}</th>
                      <th className="text-left font-semibold px-5 py-2">{t("emailverify.field.source")}</th>
                      <th className="text-left font-semibold px-5 py-2">{t("emailverify.field.purpose")}</th>
                      <th className="text-left font-semibold px-5 py-2">{t("base.field.status")}</th>
                      <th className="text-left font-semibold px-5 py-2">{t("base.field.date")}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {overview?.recent.map((row) => (
                      <tr key={row.id} className="hover:bg-slate-50">
                        <td className="px-5 py-3 text-slate-900">{row.email}</td>
                        <td className="px-5 py-3 text-slate-600 text-xs">{row.source}</td>
                        <td className="px-5 py-3 text-slate-500 text-xs">{row.purpose || "—"}</td>
                        <td className="px-5 py-3">
                          <VerificationBadge row={row} />
                        </td>
                        <td className="px-5 py-3 text-slate-500 text-xs">
                          {new Date(row.created_at).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: number | string;
  tone?: "ok" | "wait" | "off";
}) {
  const toneClass =
    tone === "ok"
      ? "text-emerald-700"
      : tone === "wait"
        ? "text-amber-700"
        : tone === "off"
          ? "text-slate-500"
          : "text-slate-900";
  return (
    <div className="bg-white border border-slate-200 rounded-xl p-4">
      <div className={`text-2xl font-bold ${toneClass}`}>{value}</div>
      <div className="text-[11px] uppercase tracking-wide text-slate-500 mt-1">{label}</div>
    </div>
  );
}

/**
 * A request whose deadline has passed is shown as expired even while the row
 * still says PENDING: the sweep that rewrites it runs on a timer, and until it
 * does, "pending" would claim somebody can still act on a dead link.
 */
function VerificationBadge({ row }: { row: EmailVerification }) {
  const { t } = useI18n();
  const expired = row.status === "EXPIRED" || (row.status === "PENDING" && new Date(row.expires_at) <= new Date());
  const state = row.status === "VERIFIED" ? "verified" : expired ? "expired" : "pending";
  const style =
    state === "verified"
      ? "bg-emerald-50 text-emerald-700 border-emerald-200"
      : state === "pending"
        ? "bg-amber-50 text-amber-700 border-amber-200"
        : "bg-slate-100 text-slate-600 border-slate-200";
  const label =
    state === "verified"
      ? t("emailverify.state.verified")
      : state === "pending"
        ? t("emailverify.state.pending")
        : t("emailverify.state.expired");
  return (
    <span className={`inline-flex items-center gap-1 text-[11px] font-bold px-2 py-0.5 rounded-full border ${style}`}>
      {state === "verified" ? <CheckCircle2 className="w-3 h-3" /> : <Clock className="w-3 h-3" />}
      {label}
    </span>
  );
}
