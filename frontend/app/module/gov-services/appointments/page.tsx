"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Appointment, GovService, gov } from "@/lib/gov";
import { useI18n } from "@/lib/i18n";
import { Banner, EmptyState, Loading, PageHeader } from "@/components/ui";
import { describeError } from "@/components/gov/shared";
import { CalendarClock, Plus } from "lucide-react";

/** In-person visit slots. Backed by /gov/appointments, which has existed since
 *  the first version of the module. */
export default function GovAppointmentsPage() {
  const { t, locale } = useI18n();
  const [appointments, setAppointments] = useState<Appointment[]>([]);
  const [services, setServices] = useState<GovService[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState({ service_id: "", citizen_name: "", scheduled_at: "", location: "", mode: "IN_PERSON" as "IN_PERSON" | "ONLINE" });

  const report = useCallback((err: unknown) => setError(describeError(err, t("base.message.error"))), [t]);

  const load = useCallback(async () => {
    try {
      const [list, svc] = await Promise.all([gov.appointments(), gov.services()]);
      setAppointments(list);
      setServices(svc);
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

  const book = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const booked = await gov.bookAppointment({
        service_id: form.service_id,
        citizen_name: form.citizen_name,
        // The API refuses a slot in the past, so send an explicit instant.
        scheduled_at: new Date(form.scheduled_at).toISOString(),
        location: form.mode === "ONLINE" ? undefined : form.location || undefined,
        mode: form.mode,
      });
      setForm({ service_id: "", citizen_name: "", scheduled_at: "", location: "", mode: "IN_PERSON" });
      // A booked appointment with no link is a partial success, not a failure:
      // say so rather than letting the row appear with a silently empty link.
      if (booked?.mode === "ONLINE" && !booked.meeting_url) {
        setError(t("gov.message.meeting_unavailable", { reason: booked.meeting_error || "" }));
      }
      await load();
    } catch (err) {
      report(err);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Loading />;

  const format = (value: string) => new Date(value).toLocaleString(locale === "en" ? "en-GB" : "mn-MN");

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarClock className="w-7 h-7 text-[var(--gerege-blue)]" />}
        title={t("gov.menu.appointments")}
        subtitle={t("gov.view.appointments_hint")}
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-4">
        <h2 className="font-semibold flex items-center gap-2">
          <Plus className="w-5 h-5 text-[var(--gerege-blue)]" />
          {t("gov.view.appointment_create")}
        </h2>
        <form onSubmit={book} className="grid sm:grid-cols-6 gap-2">
          <select
            required
            value={form.service_id}
            onChange={(e) => setForm({ ...form, service_id: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white"
          >
            <option value="">{t("gov.field.service")}</option>
            {services.map((service) => (
              <option key={service.id} value={service.id}>
                {locale === "en" && service.name_en ? service.name_en : service.name}
              </option>
            ))}
          </select>
          <input
            required
            placeholder={t("gov.field.citizen")}
            value={form.citizen_name}
            onChange={(e) => setForm({ ...form, citizen_name: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            required
            type="datetime-local"
            value={form.scheduled_at}
            onChange={(e) => setForm({ ...form, scheduled_at: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <select
            value={form.mode}
            aria-label={t("gov.field.appointment_mode")}
            onChange={(e) => setForm({ ...form, mode: e.target.value as "IN_PERSON" | "ONLINE" })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          >
            <option value="IN_PERSON">{t("gov.state.in_person")}</option>
            <option value="ONLINE">{t("gov.state.online")}</option>
          </select>
          {/* An online appointment is held over a link, so an address for it
              would be a field nobody reads. */}
          <input
            placeholder={t("gov.field.location")}
            value={form.location}
            disabled={form.mode === "ONLINE"}
            onChange={(e) => setForm({ ...form, location: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg disabled:bg-slate-100 disabled:text-slate-400"
          />
          <button
            disabled={saving}
            className="px-3 py-2 text-sm font-semibold text-white bg-[var(--gerege-blue)] rounded-lg disabled:opacity-50"
          >
            {t("gov.action.book")}
          </button>
        </form>
      </section>

      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
        {appointments.length === 0 ? (
          <EmptyState message={t("gov.message.no_appointments")} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[720px]">
              <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                <tr>
                  <th className="px-4 py-3">{t("gov.field.scheduled_at")}</th>
                  <th className="px-4 py-3">{t("gov.field.service")}</th>
                  <th className="px-4 py-3">{t("gov.field.citizen")}</th>
                  <th className="px-4 py-3">{t("gov.field.appointment_mode")}</th>
                  <th className="px-4 py-3">{t("gov.field.location")}</th>
                  <th className="px-4 py-3">{t("base.field.status")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {appointments.map((item) => (
                  <tr key={item.id}>
                    <td className="px-4 py-3">{format(item.scheduled_at)}</td>
                    <td className="px-4 py-3">{item.service_name}</td>
                    <td className="px-4 py-3">{item.citizen_name}</td>
                    <td className="px-4 py-3">
                      <span className={`text-xs font-semibold px-2 py-0.5 rounded ${
                        item.mode === "ONLINE" ? "bg-violet-100 text-violet-700" : "bg-slate-100 text-slate-600"
                      }`}>
                        {item.mode === "ONLINE" ? t("gov.state.online") : t("gov.state.in_person")}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-500">
                      {item.mode === "ONLINE" ? (
                        item.meeting_url ? (
                          <a
                            href={item.meeting_url}
                            target="_blank"
                            rel="noreferrer noopener"
                            className="text-[var(--gerege-blue)] font-medium underline"
                          >
                            {t("gov.action.join_meeting")}
                          </a>
                        ) : (
                          "—"
                        )
                      ) : (
                        item.location || "—"
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs font-semibold px-2 py-0.5 rounded bg-sky-100 text-sky-700">
                        {item.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
