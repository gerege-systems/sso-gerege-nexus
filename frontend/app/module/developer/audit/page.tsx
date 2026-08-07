"use client";

/**
 * Access audit — what this tenant's clients currently hold.
 *
 * Two questions this answers that nothing did before: which credential is
 * nobody using any more (delete it), and which user granted a client access
 * they have since forgotten about (withdraw it).
 */

import { useEffect, useState } from "react";
import { ScrollText, ShieldOff, UserMinus, Users } from "lucide-react";
import { api, type ClientActivity, type ConsentRecord } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Chip, ConfirmDialog, Empty, ErrorNote, Loading, Panel, Screen, relativeDate,
} from "../shared";

type Pending =
  | { kind: "tokens"; client: ClientActivity }
  | { kind: "consent"; consent: ConsentRecord };

export default function AccessAuditPage() {
  const { t, locale } = useI18n();
  const [clients, setClients] = useState<ClientActivity[]>([]);
  const [consents, setConsents] = useState<ConsentRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState<Pending | null>(null);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const data = await api.getDeveloperAudit();
      setClients(data.clients || []);
      setConsents(data.consents || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  async function runPending() {
    if (!pending) return;
    const target = pending;
    setPending(null);
    setError("");
    try {
      if (target.kind === "tokens") {
        const { revoked } = await api.revokeDeveloperAppTokens(target.client.client_id);
        setNotice(t("developer.audit.revoked_count", { n: revoked }));
        setTimeout(() => setNotice(""), 4000);
      } else {
        await api.withdrawDeveloperConsent(target.consent.client_id, target.consent.user_id);
      }
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  return (
    <Screen
      icon={<ScrollText className="w-5 h-5" />}
      title={t("developer.audit.title")}
      subtitle={t("developer.audit.subtitle")}
    >
      {error && <ErrorNote>{error}</ErrorNote>}
      {notice && (
        <p className="text-sm text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-lg px-3 py-2">
          {notice}
        </p>
      )}

      {loading ? (
        <Loading label={t("developer.message.loading")} />
      ) : clients.length === 0 ? (
        <Empty icon={<ScrollText className="w-9 h-9 mx-auto" />}>{t("developer.audit.no_activity")}</Empty>
      ) : (
        <Panel className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 border-b border-slate-200 text-left">
                <tr className="text-[11px] uppercase tracking-wide text-slate-500">
                  <th className="px-4 py-2.5 font-semibold">{t("developer.field.name")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("developer.audit.active_access")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("developer.audit.active_refresh")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("developer.audit.consented")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("developer.field.last_used")}</th>
                  <th className="px-4 py-2.5" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {clients.map((client) => {
                  const live = client.active_access_tokens + client.active_refresh_tokens;
                  return (
                    <tr key={client.client_id} className="hover:bg-slate-50/60">
                      <td className="px-4 py-3">
                        <div className="font-semibold text-slate-900 flex items-center gap-2">
                          {client.client_name}
                          {client.disabled && <Chip tone="rose">{t("developer.message.disabled")}</Chip>}
                        </div>
                        <div className="text-[11px] font-mono text-slate-400">{client.client_id}</div>
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums font-semibold text-slate-900">
                        {client.active_access_tokens}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums font-semibold text-slate-900">
                        {client.active_refresh_tokens}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-slate-600">{client.consented_users}</td>
                      <td className="px-4 py-3 text-slate-500 text-xs">
                        {relativeDate(client.last_used_at, t("developer.message.never_used"), locale)}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => setPending({ kind: "tokens", client })}
                          disabled={live === 0}
                          className="text-xs font-semibold text-rose-700 hover:bg-rose-50 px-2.5 py-1.5 rounded-lg inline-flex items-center gap-1.5 disabled:opacity-30 disabled:hover:bg-transparent"
                        >
                          <ShieldOff className="w-3.5 h-3.5" /> {t("developer.audit.revoke_tokens")}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Panel>
      )}

      <section className="space-y-3">
        <h2 className="text-sm font-bold text-slate-900 flex items-center gap-2">
          <Users className="w-4 h-4 text-slate-400" /> {t("developer.audit.consents_title")}
        </h2>
        {loading ? null : consents.length === 0 ? (
          <Empty icon={<Users className="w-9 h-9 mx-auto" />}>{t("developer.audit.no_consents")}</Empty>
        ) : (
          <Panel className="divide-y divide-slate-100">
            {consents.map((consent) => (
              <div key={`${consent.client_id}:${consent.user_id}`} className="p-4 flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold text-slate-900">
                    {consent.user_name || consent.user_email}
                    <span className="font-normal text-slate-400"> → </span>
                    {consent.client_name}
                  </p>
                  <p className="text-[11px] text-slate-400">
                    {consent.user_email} · {new Date(consent.granted_at).toLocaleDateString()}
                  </p>
                  <div className="flex flex-wrap gap-1 mt-1.5">
                    {consent.scopes.map((scope) => <Chip key={scope} mono tone="blue">{scope}</Chip>)}
                  </div>
                </div>
                <button
                  onClick={() => setPending({ kind: "consent", consent })}
                  className="text-xs font-semibold text-rose-700 hover:bg-rose-50 px-3 py-1.5 rounded-lg flex items-center gap-1.5"
                >
                  <UserMinus className="w-3.5 h-3.5" /> {t("developer.audit.withdraw")}
                </button>
              </div>
            ))}
          </Panel>
        )}
      </section>

      {pending && (
        <ConfirmDialog
          danger
          title={pending.kind === "tokens" ? pending.client.client_name : pending.consent.user_email}
          body={pending.kind === "tokens" ? t("developer.audit.revoke_warning") : t("developer.audit.withdraw_warning")}
          confirmLabel={pending.kind === "tokens" ? t("developer.audit.revoke_tokens") : t("developer.audit.withdraw")}
          onCancel={() => setPending(null)}
          onConfirm={runPending}
        />
      )}
    </Screen>
  );
}
