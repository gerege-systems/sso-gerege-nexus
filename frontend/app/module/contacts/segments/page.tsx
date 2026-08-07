"use client";

/**
 * Contact segments.
 *
 * Derived, not stored. The contacts table holds a company, an active flag, an
 * email and a phone, so those are the cuts this can honestly offer — there is
 * no segment builder behind it and the screen does not pretend otherwise.
 */

import { useEffect, useMemo, useState } from "react";
import { Building2, MailX, Users } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Panel, Screen } from "@/components/module/kit";

type Contact = { id: string; name: string; email: string; phone: string; company: string; active: boolean };

export default function ContactSegmentsPage() {
  const { t } = useI18n();
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setContacts((await api.getContacts()) || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const byCompany = useMemo(() => {
    const groups = new Map<string, Contact[]>();
    for (const contact of contacts) {
      const key = contact.company.trim() || "";
      groups.set(key, [...(groups.get(key) || []), contact]);
    }
    return [...groups.entries()].sort((a, b) => b[1].length - a[1].length);
  }, [contacts]);

  const active = contacts.filter((c) => c.active);
  const reachable = contacts.filter((c) => c.email.trim() || c.phone.trim());

  return (
    <Screen icon={<Users className="w-5 h-5" />} title={t("mod.contacts.segments.title")} subtitle={t("mod.contacts.segments.subtitle")}>
      {error && <ErrorNote>{error}</ErrorNote>}
      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : contacts.length === 0 ? (
        <Empty icon={<Users className="w-9 h-9 mx-auto" />}>{t("base.message.no_data")}</Empty>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-3">
            <Stat label={t("mod.contacts.segments.active")} value={active.length} total={contacts.length} />
            <Stat label={t("mod.contacts.segments.inactive")} value={contacts.length - active.length} total={contacts.length} />
            <Stat
              label={t("mod.contacts.segments.reachable")}
              value={reachable.length}
              total={contacts.length}
              hint={reachable.length === contacts.length ? t("mod.contacts.segments.reachable_hint") : t("mod.contacts.segments.unreachable_hint")}
            />
          </div>

          <Panel className="overflow-hidden">
            <h2 className="text-sm font-bold text-slate-900 px-4 py-3 border-b border-slate-100 flex items-center gap-2">
              <Building2 className="w-4 h-4 text-slate-400" /> {t("mod.contacts.segments.by_company")}
            </h2>
            <ul className="divide-y divide-slate-100">
              {byCompany.map(([company, members]) => (
                <li key={company || "none"} className="px-4 py-3 flex items-center gap-3">
                  <span className={`flex-1 text-sm ${company ? "text-slate-900 font-medium" : "text-slate-400 italic"}`}>
                    {company || t("mod.contacts.segments.no_company")}
                  </span>
                  {!company && <MailX className="w-3.5 h-3.5 text-slate-300" />}
                  <Chip>{members.length}</Chip>
                  <span className="w-32 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                    <span
                      className="block h-full bg-indigo-500"
                      style={{ width: `${Math.round((members.length / contacts.length) * 100)}%` }}
                    />
                  </span>
                </li>
              ))}
            </ul>
          </Panel>
        </>
      )}
    </Screen>
  );
}

function Stat({ label, value, total, hint }: { label: string; value: number; total: number; hint?: string }) {
  return (
    <Panel className="p-4">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="text-2xl font-bold text-slate-900 tabular-nums mt-1">
        {value}
        <span className="text-sm font-normal text-slate-400"> / {total}</span>
      </p>
      {hint && <p className="text-[11px] text-slate-400 mt-1">{hint}</p>}
    </Panel>
  );
}
