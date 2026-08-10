"use client";

import React, { useCallback, useEffect, useState } from "react";
import { FulfillmentMode, GovService, OrgUnit, Workflow, gov } from "@/lib/gov";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, EmptyState, Loading, PageHeader } from "@/components/ui";
import { describeError } from "@/components/gov/shared";
import { Landmark, Plus } from "lucide-react";

/** Service passport registry: what the organisation delivers, and how each
 *  service is fulfilled. Backed by /gov/services and
 *  /gov/services/{id}/configuration. */
export default function GovCatalogPage() {
  const { t, locale } = useI18n();
  // A new service passport is organisation-wide, so the backend asks for the
  // tenant administrator role; changing how a service is fulfilled asks for
  // gov.configure. The screen mirrors both instead of offering a 403.
  const access = useAccess();
  const mayConfigure = access.can("gov.configure");
  const [services, setServices] = useState<GovService[]>([]);
  const [units, setUnits] = useState<OrgUnit[]>([]);
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ code: "", name: "", category: "", fee: "", duration_days: "" });

  const report = useCallback((err: unknown) => setError(describeError(err, t("base.message.error"))), [t]);

  const load = useCallback(async () => {
    try {
      const [svc, unitList, flows] = await Promise.all([gov.services(), gov.units(), gov.workflows()]);
      setServices(svc);
      setUnits(unitList);
      setWorkflows(flows);
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

  const publishedVersions = workflows.flatMap((wf) =>
    (wf.versions || [])
      .filter((v) => v.status === "PUBLISHED")
      .map((v) => ({ id: v.id, label: `${wf.code} v${v.version}` })),
  );

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await gov.createService({
        code: form.code,
        name: form.name,
        category: form.category || undefined,
        fee: form.fee ? Number(form.fee) : 0,
        duration_days: form.duration_days ? Number(form.duration_days) : 1,
      });
      setForm({ code: "", name: "", category: "", fee: "", duration_days: "" });
      setCreating(false);
      await load();
    } catch (err) {
      report(err);
    } finally {
      setSaving(false);
    }
  };

  const configure = async (
    service: GovService,
    patch: Partial<Pick<GovService, "fulfillment_mode" | "workflow_version_id" | "owner_unit_id">>,
  ) => {
    setSaving(true);
    setError(null);
    try {
      await gov.configureService(service.id, {
        fulfillment_mode: (patch.fulfillment_mode ?? service.fulfillment_mode) as FulfillmentMode,
        workflow_version_id: patch.workflow_version_id ?? service.workflow_version_id ?? null,
        owner_unit_id: patch.owner_unit_id ?? service.owner_unit_id ?? null,
      });
      await load();
    } catch (err) {
      report(err);
    } finally {
      setSaving(false);
    }
  };

  if (loading || !access.ready) return <Loading />;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Landmark className="w-7 h-7 text-[var(--gerege-blue)]" />}
        title={t("gov.menu.catalog")}
        subtitle={t("gov.view.catalog_hint")}
        actions={
          access.isAdmin ? (
            <button
              onClick={() => setCreating((v) => !v)}
              className="flex items-center gap-2 px-3 py-2 text-sm font-semibold text-white bg-[var(--gerege-blue)] rounded-lg"
            >
              <Plus className="w-4 h-4" />
              {t("base.action.create")}
            </button>
          ) : undefined
        }
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      {creating && (
        <form onSubmit={create} className="bg-white border border-slate-200 rounded-xl p-5 grid sm:grid-cols-5 gap-2">
          <input
            required
            placeholder={t("gov.field.unit_code")}
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            required
            placeholder={t("base.field.name")}
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            placeholder={t("gov.field.service")}
            value={form.category}
            onChange={(e) => setForm({ ...form, category: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            type="number"
            min={0}
            placeholder={t("gov.field.fee")}
            value={form.fee}
            onChange={(e) => setForm({ ...form, fee: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <button
            disabled={saving}
            className="px-3 py-2 text-sm font-semibold text-white bg-[var(--gerege-blue)] rounded-lg disabled:opacity-50"
          >
            {t("base.action.save")}
          </button>
        </form>
      )}

      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
        {services.length === 0 ? (
          <EmptyState message={t("gov.message.no_services")} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[860px]">
              <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">{t("gov.field.service")}</th>
                  <th className="px-3 py-2">{t("gov.field.fee")}</th>
                  <th className="px-3 py-2">{t("gov.field.mode")}</th>
                  <th className="px-3 py-2">{t("gov.field.workflow_version")}</th>
                  <th className="px-3 py-2">{t("gov.field.owner_unit")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {services.map((service) => (
                  <tr key={service.id}>
                    <td className="px-3 py-2">
                      <div className="font-medium">
                        {locale === "en" && service.name_en ? service.name_en : service.name}
                      </div>
                      <div className="text-xs text-slate-400 font-mono">{service.code}</div>
                    </td>
                    <td className="px-3 py-2 text-slate-600">
                      {new Intl.NumberFormat(locale === "en" ? "en-US" : "mn-MN").format(service.fee)}₮
                    </td>
                    <td className="px-3 py-2">
                      <select
                        value={service.fulfillment_mode}
                        disabled={!mayConfigure}
                        onChange={(e) => configure(service, { fulfillment_mode: e.target.value as FulfillmentMode })}
                        className="px-2 py-1 text-xs border border-slate-300 rounded bg-white disabled:bg-slate-50 disabled:text-slate-500"
                      >
                        {(["LOCAL", "DELEGATE", "HYBRID"] as const).map((mode) => (
                          <option key={mode} value={mode}>
                            {mode}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-3 py-2">
                      <select
                        value={service.workflow_version_id || ""}
                        disabled={!mayConfigure}
                        onChange={(e) => configure(service, { workflow_version_id: e.target.value || null })}
                        className="px-2 py-1 text-xs border border-slate-300 rounded bg-white disabled:bg-slate-50 disabled:text-slate-500"
                      >
                        <option value="">—</option>
                        {publishedVersions.map((version) => (
                          <option key={version.id} value={version.id}>
                            {version.label}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-3 py-2">
                      <select
                        value={service.owner_unit_id || ""}
                        disabled={!mayConfigure}
                        onChange={(e) => configure(service, { owner_unit_id: e.target.value || null })}
                        className="px-2 py-1 text-xs border border-slate-300 rounded bg-white disabled:bg-slate-50 disabled:text-slate-500"
                      >
                        <option value="">—</option>
                        {units.map((unit) => (
                          <option key={unit.id} value={unit.id}>
                            {unit.code}
                          </option>
                        ))}
                      </select>
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
