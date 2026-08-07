"use client";

/**
 * Redirect policies — every registered callback, and the rules behind them.
 *
 * The authorization endpoint matches a redirect_uri exactly against this list,
 * so the list is the security boundary. Seeing it in one place is how you
 * notice the loopback URI somebody left on a production integration.
 */

import { useEffect, useMemo, useState } from "react";
import { Globe, Laptop, Route, Server, ShieldCheck, Smartphone } from "lucide-react";
import { api, type OAuth2Client } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, CopyButton, Empty, ErrorNote, Loading, Panel, Screen, useCopy } from "../shared";

type Entry = { client: OAuth2Client; uri: string; kind: "https" | "loopback" | "custom" };

function classify(uri: string): Entry["kind"] {
  try {
    const parsed = new URL(uri);
    if (parsed.protocol === "https:") return "https";
    if (parsed.protocol === "http:") return "loopback";
    return "custom";
  } catch {
    return "custom";
  }
}

export default function RedirectPoliciesPage() {
  const { t } = useI18n();
  const { copied, copy } = useCopy();
  const [clients, setClients] = useState<OAuth2Client[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setClients((await api.getDeveloperApps()) || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const entries = useMemo<Entry[]>(
    () => clients.flatMap((client) => client.redirect_uris.map((uri) => ({ client, uri, kind: classify(uri) }))),
    [clients],
  );

  // A client registered only for client_credentials never receives a redirect,
  // so its absence from the list below is correct rather than a gap.
  const machineOnly = clients.filter((c) => c.redirect_uris.length === 0);

  const rules = [
    { icon: <ShieldCheck className="w-4 h-4" />, text: t("developer.redirects.rule_exact") },
    { icon: <Globe className="w-4 h-4" />, text: t("developer.redirects.rule_https") },
    { icon: <Route className="w-4 h-4" />, text: t("developer.redirects.rule_fragment") },
    { icon: <Smartphone className="w-4 h-4" />, text: t("developer.redirects.rule_custom") },
  ];

  return (
    <Screen
      icon={<Route className="w-5 h-5" />}
      title={t("developer.redirects.title")}
      subtitle={t("developer.redirects.subtitle")}
    >
      {error && <ErrorNote>{error}</ErrorNote>}

      <Panel className="p-5">
        <h2 className="text-sm font-bold text-slate-900 mb-3">{t("developer.redirects.rules_title")}</h2>
        <ul className="space-y-2.5">
          {rules.map((rule, index) => (
            <li key={index} className="flex items-start gap-2.5 text-xs text-slate-600">
              <span className="text-indigo-600 shrink-0 mt-0.5">{rule.icon}</span>
              {rule.text}
            </li>
          ))}
        </ul>
      </Panel>

      {loading ? (
        <Loading label={t("developer.message.loading")} />
      ) : entries.length === 0 ? (
        <Empty icon={<Route className="w-9 h-9 mx-auto" />}>{t("developer.redirects.none")}</Empty>
      ) : (
        <Panel className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 border-b border-slate-200 text-left">
                <tr className="text-[11px] uppercase tracking-wide text-slate-500">
                  <th className="px-4 py-2.5 font-semibold">{t("developer.field.name")}</th>
                  <th className="px-4 py-2.5 font-semibold">redirect_uri</th>
                  <th className="px-4 py-2.5 font-semibold">{t("base.field.type")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {entries.map(({ client, uri, kind }) => (
                  <tr key={`${client.client_id}:${uri}`} className="hover:bg-slate-50/60">
                    <td className="px-4 py-3">
                      <div className="font-semibold text-slate-900 flex items-center gap-1.5">
                        {client.client_type === "public"
                          ? <Smartphone className="w-3.5 h-3.5 text-slate-400" />
                          : <Server className="w-3.5 h-3.5 text-slate-400" />}
                        {client.client_name}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <code className="text-xs font-mono text-slate-700 break-all">{uri}</code>
                        <CopyButton value={uri} id={`${client.client_id}:${uri}`} copied={copied} onCopy={copy} />
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      {kind === "https" && <Chip tone="emerald">https</Chip>}
                      {kind === "loopback" && (
                        <span className="inline-flex items-center gap-1">
                          <Laptop className="w-3.5 h-3.5 text-amber-600" />
                          <Chip tone="amber">{t("developer.redirects.loopback")}</Chip>
                        </span>
                      )}
                      {kind === "custom" && <Chip tone="blue">custom scheme</Chip>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}

      {machineOnly.length > 0 && (
        <Panel className="p-4">
          <p className="text-[11px] font-semibold text-slate-500 mb-2">
            {t("developer.redirects.no_redirect_needed")}
          </p>
          <div className="flex flex-wrap gap-1">
            {machineOnly.map((client) => <Chip key={client.client_id}>{client.client_name}</Chip>)}
          </div>
        </Panel>
      )}
    </Screen>
  );
}
