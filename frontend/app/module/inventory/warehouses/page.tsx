"use client";

/**
 * Warehouses.
 *
 * The list and create endpoints have existed since the inventory app shipped;
 * there was simply no screen calling them. Stock lines are counted per
 * warehouse from the stock-levels feed so an empty location is obvious.
 */

import { useEffect, useMemo, useState } from "react";
import { Plus, Warehouse } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Modal, Panel, Screen } from "@/components/module/kit";

type WarehouseRow = { id: string; code: string; name: string; address: string; created_at: string };

export default function WarehousesPage() {
  const { t } = useI18n();
  const [warehouses, setWarehouses] = useState<WarehouseRow[]>([]);
  const [lines, setLines] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState({ code: "", name: "", address: "" });

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [list, stock] = await Promise.all([api.getWarehouses(), api.getStockLevels()]);
      setWarehouses(list || []);
      const counts: Record<string, number> = {};
      for (const level of stock || []) counts[level.warehouse_id] = (counts[level.warehouse_id] || 0) + 1;
      setLines(counts);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await api.createWarehouse(draft);
      setDraft({ code: "", name: "", address: "" });
      setAdding(false);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  return (
    <Screen
      icon={<Warehouse className="w-5 h-5" />}
      title={t("mod.inventory.warehouses.title")}
      subtitle={t("mod.inventory.warehouses.subtitle")}
      action={
        <button onClick={() => setAdding(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white px-4 py-2 rounded-lg text-sm font-semibold flex items-center gap-2 shadow-sm">
          <Plus className="w-4 h-4" /> {t("mod.inventory.warehouses.add")}
        </button>
      }
    >
      {error && <ErrorNote>{error}</ErrorNote>}

      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : warehouses.length === 0 ? (
        <Empty icon={<Warehouse className="w-9 h-9 mx-auto" />}>{t("mod.inventory.warehouses.none")}</Empty>
      ) : (
        <div className="grid gap-3 md:grid-cols-2">
          {warehouses.map((warehouse) => (
            <Panel key={warehouse.id} className="p-4">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <h3 className="font-bold text-slate-900">{warehouse.name}</h3>
                  <code className="text-xs font-mono text-slate-500">{warehouse.code}</code>
                </div>
                <Chip tone={lines[warehouse.id] ? "blue" : "slate"}>
                  {lines[warehouse.id] || 0} {t("mod.inventory.warehouses.lines")}
                </Chip>
              </div>
              {warehouse.address && <p className="text-xs text-slate-500 mt-2">{warehouse.address}</p>}
            </Panel>
          ))}
        </div>
      )}

      {adding && (
        <Modal onClose={() => setAdding(false)}>
          <form onSubmit={create} className="space-y-4">
            <h2 className="text-lg font-bold text-slate-900">{t("mod.inventory.warehouses.add")}</h2>
            {([["code", t("mod.inventory.warehouses.code")], ["name", t("base.field.name")], ["address", t("mod.inventory.warehouses.address")]] as const).map(([field, label]) => (
              <label key={field} className="block">
                <span className="text-xs font-semibold text-slate-700">{label}{field !== "address" ? " *" : ""}</span>
                <input
                  value={draft[field]}
                  onChange={(e) => setDraft({ ...draft, [field]: e.target.value })}
                  required={field !== "address"}
                  className="mt-1 w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 outline-none"
                />
              </label>
            ))}
            <div className="flex justify-end gap-2 pt-2">
              <button type="button" onClick={() => setAdding(false)} className="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-lg">
                {t("base.action.cancel")}
              </button>
              <button type="submit" className="px-4 py-2 text-sm bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg font-semibold">
                {t("base.action.create")}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </Screen>
  );
}
