"use client";

import React, { useCallback, useEffect, useState } from "react";
import { GovService, OrgUnit, RoutingRule, RoutingStrategy, Workflow, WorkflowTemplate, gov } from "@/lib/gov";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, Loading, PageHeader } from "@/components/ui";
import { describeError } from "@/components/gov/shared";
import { Building2, Route, Workflow as WorkflowIcon } from "lucide-react";

const STRATEGIES: RoutingStrategy[] = ["SELF", "PARENT", "CHILD", "SPECIFIC_UNIT", "REGION_MATCH"];

/** Decision workflow: the organisational hierarchy, published workflow
 *  versions and the routing rules that pick an executing unit. */
export default function GovWorkflowPage() {
  const { t, locale } = useI18n();
  // gov.configure is granted to administrators only by default; everyone else
  // reads the configuration without the forms that would 403.
  const access = useAccess();
  const mayConfigure = access.can("gov.configure");
  const [units, setUnits] = useState<OrgUnit[]>([]);
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [templates, setTemplates] = useState<WorkflowTemplate[]>([]);
  const [rules, setRules] = useState<RoutingRule[]>([]);
  const [services, setServices] = useState<GovService[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [unitForm, setUnitForm] = useState({ code: "", name: "", parent_id: "", region_code: "" });
  const [template, setTemplate] = useState("");
  const [ruleForm, setRuleForm] = useState({
    strategy: "SELF" as RoutingStrategy,
    service_id: "",
    priority: "100",
    match_field: "",
    match_value: "",
    target_unit_id: "",
  });

  const report = useCallback((err: unknown) => setError(describeError(err, t("base.message.error"))), [t]);

  const load = useCallback(async () => {
    try {
      const [unitList, flows, tpl, ruleList, svc] = await Promise.all([
        gov.units(),
        gov.workflows(),
        gov.templates(),
        gov.routingRules(),
        gov.services(),
      ]);
      setUnits(unitList);
      setWorkflows(flows);
      setTemplates(tpl);
      setRules(ruleList);
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

  const submitUnit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await gov.createUnit({
        code: unitForm.code,
        name: unitForm.name,
        parent_id: unitForm.parent_id || null,
        region_code: unitForm.region_code || undefined,
      });
      setUnitForm({ code: "", name: "", parent_id: "", region_code: "" });
      await load();
    } catch (err) {
      report(err);
    } finally {
      setSaving(false);
    }
  };

  // Instantiating a template publishes it: a service may only point at a
  // published version.
  const instantiate = async () => {
    if (!template) return;
    setSaving(true);
    setError(null);
    try {
      const version = await gov.createWorkflow({ template });
      await gov.publishVersion(version.id);
      setTemplate("");
      await load();
    } catch (err) {
      report(err);
    } finally {
      setSaving(false);
    }
  };

  const submitRule = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await gov.createRoutingRule({
        strategy: ruleForm.strategy,
        service_id: ruleForm.service_id || null,
        priority: Number(ruleForm.priority) || 100,
        match_field: ruleForm.match_field || undefined,
        match_value: ruleForm.match_value || undefined,
        target_unit_id: ruleForm.target_unit_id || null,
      });
      setRuleForm({ strategy: "SELF", service_id: "", priority: "100", match_field: "", match_value: "", target_unit_id: "" });
      await load();
    } catch (err) {
      report(err);
    } finally {
      setSaving(false);
    }
  };

  if (loading || !access.ready) return <Loading />;

  const unitCode = (id?: string | null) => (id ? units.find((u) => u.id === id)?.code || id : "—");
  const serviceName = (id?: string | null) =>
    id ? services.find((s) => s.id === id)?.name || id : t("gov.filter.all_statuses");

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<WorkflowIcon className="w-7 h-7 text-[var(--gerege-blue)]" />}
        title={t("gov.menu.workflow")}
        subtitle={t("gov.view.workflow_hint")}
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-4">
        <h2 className="font-semibold flex items-center gap-2">
          <Building2 className="w-5 h-5 text-[var(--gerege-blue)]" />
          {t("gov.view.units")}
        </h2>
        <ul className="text-sm divide-y divide-slate-100">
          {units.length === 0 && <li className="py-2 text-slate-500">{t("gov.message.no_units")}</li>}
          {units.map((unit) => (
            <li key={unit.id} className="py-2 flex items-center gap-2">
              <span className="font-mono text-xs text-slate-400 w-16">{unit.code}</span>
              <span className="flex-1">{locale === "en" && unit.name_en ? unit.name_en : unit.name}</span>
              {unit.parent_id && (
                <span className="text-xs text-slate-400">← {unitCode(unit.parent_id)}</span>
              )}
              <span className="text-xs text-slate-400">{unit.unit_type}</span>
            </li>
          ))}
        </ul>

        {mayConfigure && (
        <form onSubmit={submitUnit} className="grid sm:grid-cols-5 gap-2">
          <input
            required
            placeholder={t("gov.field.unit_code")}
            value={unitForm.code}
            onChange={(e) => setUnitForm({ ...unitForm, code: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            required
            placeholder={t("base.field.name")}
            value={unitForm.name}
            onChange={(e) => setUnitForm({ ...unitForm, name: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <select
            value={unitForm.parent_id}
            onChange={(e) => setUnitForm({ ...unitForm, parent_id: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white"
          >
            <option value="">{t("gov.message.no_parent")}</option>
            {units.map((unit) => (
              <option key={unit.id} value={unit.id}>
                {unit.code}
              </option>
            ))}
          </select>
          <input
            placeholder="region"
            value={unitForm.region_code}
            onChange={(e) => setUnitForm({ ...unitForm, region_code: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <button
            disabled={saving}
            className="px-3 py-2 text-sm font-semibold text-white bg-[var(--gerege-blue)] rounded-lg disabled:opacity-50"
          >
            {t("base.action.create")}
          </button>
        </form>
        )}
      </section>

      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-4">
        <h2 className="font-semibold flex items-center gap-2">
          <WorkflowIcon className="w-5 h-5 text-[var(--gerege-blue)]" />
          {t("gov.view.workflows")}
        </h2>
        <ul className="text-sm divide-y divide-slate-100">
          {workflows.length === 0 && <li className="py-2 text-slate-500">{t("gov.message.no_workflows")}</li>}
          {workflows.map((wf) => (
            <li key={wf.id} className="py-2">
              <span className="font-medium">{locale === "en" && wf.name_en ? wf.name_en : wf.name}</span>
              <span className="ml-2 text-xs text-slate-400 font-mono">{wf.code}</span>
              <div className="flex flex-wrap gap-1.5 mt-1">
                {(wf.versions || []).map((version) => (
                  <span
                    key={version.id}
                    className={`text-xs px-2 py-0.5 rounded ${
                      version.status === "PUBLISHED" ? "bg-emerald-100 text-emerald-700" : "bg-slate-100 text-slate-600"
                    }`}
                  >
                    v{version.version} · {version.status}
                  </span>
                ))}
              </div>
            </li>
          ))}
        </ul>

        {mayConfigure && (
        <div className="flex flex-wrap gap-2">
          <select
            value={template}
            onChange={(e) => setTemplate(e.target.value)}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white flex-1 min-w-52"
          >
            <option value="">{t("gov.action.pick_template")}</option>
            {templates.map((tpl) => (
              <option key={tpl.Code} value={tpl.Code}>
                {locale === "en" ? tpl.NameEN : tpl.Name}
              </option>
            ))}
          </select>
          <button
            onClick={instantiate}
            disabled={!template || saving}
            className="px-3 py-2 text-sm font-semibold text-white bg-[var(--gerege-blue)] rounded-lg disabled:opacity-50"
          >
            {t("gov.action.publish")}
          </button>
        </div>
        )}
      </section>

      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-4">
        <h2 className="font-semibold flex items-center gap-2">
          <Route className="w-5 h-5 text-[var(--gerege-blue)]" />
          {t("gov.view.routing_rules")}
        </h2>
        {rules.length === 0 ? (
          <p className="text-sm text-slate-500">{t("gov.message.no_routing_rules")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[640px]">
              <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">{t("gov.field.priority")}</th>
                  <th className="px-3 py-2">{t("gov.field.service")}</th>
                  <th className="px-3 py-2">{t("gov.field.match")}</th>
                  <th className="px-3 py-2">{t("gov.field.strategy")}</th>
                  <th className="px-3 py-2">{t("gov.field.target_unit")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {rules.map((rule) => (
                  <tr key={rule.id}>
                    <td className="px-3 py-2">{rule.priority}</td>
                    <td className="px-3 py-2">{serviceName(rule.service_id)}</td>
                    <td className="px-3 py-2 text-slate-500">
                      {rule.match_field ? `${rule.match_field} = ${rule.match_value}` : "—"}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs">{rule.strategy}</td>
                    <td className="px-3 py-2">{unitCode(rule.target_unit_id)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {mayConfigure && (
        <form onSubmit={submitRule} className="grid sm:grid-cols-6 gap-2">
          <select
            value={ruleForm.strategy}
            onChange={(e) => setRuleForm({ ...ruleForm, strategy: e.target.value as RoutingStrategy })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white"
          >
            {STRATEGIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
          <select
            value={ruleForm.service_id}
            onChange={(e) => setRuleForm({ ...ruleForm, service_id: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white"
          >
            <option value="">{t("gov.filter.all_statuses")}</option>
            {services.map((service) => (
              <option key={service.id} value={service.id}>
                {service.code}
              </option>
            ))}
          </select>
          <input
            type="number"
            value={ruleForm.priority}
            onChange={(e) => setRuleForm({ ...ruleForm, priority: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            placeholder="field"
            value={ruleForm.match_field}
            onChange={(e) => setRuleForm({ ...ruleForm, match_field: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <input
            placeholder="value"
            value={ruleForm.match_value}
            onChange={(e) => setRuleForm({ ...ruleForm, match_value: e.target.value })}
            className="px-3 py-2 text-sm border border-slate-300 rounded-lg"
          />
          <div className="flex gap-2">
            <select
              value={ruleForm.target_unit_id}
              onChange={(e) => setRuleForm({ ...ruleForm, target_unit_id: e.target.value })}
              className="px-2 py-2 text-sm border border-slate-300 rounded-lg bg-white flex-1"
            >
              <option value="">—</option>
              {units.map((unit) => (
                <option key={unit.id} value={unit.id}>
                  {unit.code}
                </option>
              ))}
            </select>
            <button
              disabled={saving}
              className="px-3 py-2 text-sm font-semibold text-white bg-[var(--gerege-blue)] rounded-lg disabled:opacity-50"
            >
              +
            </button>
          </div>
        </form>
        )}
      </section>
    </div>
  );
}
