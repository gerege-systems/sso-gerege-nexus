"use client";

/**
 * Duplicate contacts.
 *
 * Points, never merges. Deciding which of two records survives needs judgement
 * the screen does not have — and there is no merge endpoint to call even if it
 * did. Saying so is better than a button that silently picks the newer row.
 */

import { useEffect, useMemo, useState } from "react";
import { Copy, Mail, Phone, User } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Panel, Screen } from "@/components/module/kit";

type Contact = { id: string; name: string; email: string; phone: string; company: string; active: boolean };
type Cluster = { key: string; field: "email" | "phone" | "name"; members: Contact[] };

/** normalise strips the differences that should not make two records distinct. */
function normalise(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, " ");
}

export default function ContactDuplicatesPage() {
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

  const clusters = useMemo<Cluster[]>(() => {
    const found: Cluster[] = [];
    // A contact can collide on more than one field; each collision is listed
    // once, and the strongest signal comes first.
    const fields: Cluster["field"][] = ["email", "phone", "name"];
    const alreadyPaired = new Set<string>();

    for (const field of fields) {
      const buckets = new Map<string, Contact[]>();
      for (const contact of contacts) {
        const raw = field === "phone" ? contact.phone.replace(/[^0-9]/g, "") : contact[field];
        const key = normalise(raw || "");
        if (!key) continue;
        buckets.set(key, [...(buckets.get(key) || []), contact]);
      }
      for (const [key, members] of buckets) {
        if (members.length < 2) continue;
        const signature = members.map((m) => m.id).sort().join("|");
        if (alreadyPaired.has(signature)) continue;
        alreadyPaired.add(signature);
        found.push({ key, field, members });
      }
    }
    return found;
  }, [contacts]);

  const icons = { email: <Mail className="w-3.5 h-3.5" />, phone: <Phone className="w-3.5 h-3.5" />, name: <User className="w-3.5 h-3.5" /> };

  return (
    <Screen icon={<Copy className="w-5 h-5" />} title={t("mod.contacts.duplicates.title")} subtitle={t("mod.contacts.duplicates.subtitle")}>
      {error && <ErrorNote>{error}</ErrorNote>}
      <Panel className="p-4 bg-slate-50">
        <p className="text-xs text-slate-600">{t("mod.contacts.duplicates.note")}</p>
      </Panel>

      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : clusters.length === 0 ? (
        <Empty icon={<Copy className="w-9 h-9 mx-auto" />}>{t("mod.contacts.duplicates.none")}</Empty>
      ) : (
        <div className="space-y-3">
          {clusters.map((cluster) => (
            <Panel key={`${cluster.field}:${cluster.key}`} className="p-4">
              <div className="flex items-center gap-2 mb-3">
                <span className="text-[11px] font-semibold text-slate-500 flex items-center gap-1.5">
                  {icons[cluster.field]} {t("mod.contacts.duplicates.by")}: {cluster.field}
                </span>
                <code className="text-xs font-mono text-slate-700 bg-slate-100 px-2 py-0.5 rounded">{cluster.key}</code>
                <Chip tone="amber">{t("mod.contacts.duplicates.count", { n: cluster.members.length })}</Chip>
              </div>
              <ul className="divide-y divide-slate-100">
                {cluster.members.map((contact) => (
                  <li key={contact.id} className="py-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                    <span className="font-medium text-slate-900">{contact.name}</span>
                    {contact.company && <span className="text-slate-500 text-xs">{contact.company}</span>}
                    <span className="text-xs font-mono text-slate-400">{contact.email || "—"}</span>
                    <span className="text-xs font-mono text-slate-400">{contact.phone || "—"}</span>
                    {!contact.active && <Chip>{t("mod.contacts.segments.inactive")}</Chip>}
                  </li>
                ))}
              </ul>
            </Panel>
          ))}
        </div>
      )}
    </Screen>
  );
}
