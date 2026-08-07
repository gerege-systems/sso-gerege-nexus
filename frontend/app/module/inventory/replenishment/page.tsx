"use client";

/**
 * Replenishment.
 *
 * There is no per-product reorder point in the schema, so the threshold is a
 * control on this screen rather than a stored policy — and the note says so.
 * Inventing a reorder_point column would be a bigger claim than the data
 * supports.
 */

import { useEffect, useMemo, useState } from "react";
import { PackageX, RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, Empty, ErrorNote, Loading, Panel, Screen } from "@/components/module/kit";

export default function ReplenishmentPage() {
  const { t } = useI18n();
  const [levels, setLevels] = useState<Array<{ id: string; warehouse_id: string; product_id: string; quantity: number }>>([]);
  const [products, setProducts] = useState<Record<string, string>>({});
  const [warehouses, setWarehouses] = useState<Record<string, string>>({});
  const [threshold, setThreshold] = useState(10);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        const [stock, productList, warehouseList] = await Promise.all([
          api.getStockLevels(), api.getProducts(), api.getWarehouses(),
        ]);
        setLevels(stock || []);
        setProducts(Object.fromEntries((productList || []).map((p) => [p.id, `${p.sku} · ${p.name}`])));
        setWarehouses(Object.fromEntries((warehouseList || []).map((w) => [w.id, w.name])));
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const low = useMemo(
    () => levels.filter((l) => Number(l.quantity) <= threshold).sort((a, b) => Number(a.quantity) - Number(b.quantity)),
    [levels, threshold],
  );

  return (
    <Screen
      icon={<RefreshCw className="w-5 h-5" />}
      title={t("mod.inventory.replenishment.title")}
      subtitle={t("mod.inventory.replenishment.subtitle")}
      action={
        <label className="flex items-center gap-2 text-sm">
          <span className="text-xs font-semibold text-slate-600">{t("mod.inventory.replenishment.threshold")}</span>
          <input
            type="number"
            min={0}
            value={threshold}
            onChange={(e) => setThreshold(Math.max(0, Number(e.target.value)))}
            className="w-20 px-2 py-1.5 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 outline-none"
          />
        </label>
      }
    >
      {error && <ErrorNote>{error}</ErrorNote>}
      <Panel className="p-4 bg-slate-50">
        <p className="text-xs text-slate-600">{t("mod.inventory.replenishment.note")}</p>
      </Panel>

      {loading ? (
        <Loading label={t("base.message.loading")} />
      ) : low.length === 0 ? (
        <Empty icon={<PackageX className="w-9 h-9 mx-auto" />}>{t("mod.inventory.replenishment.ok")}</Empty>
      ) : (
        <Panel className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 border-b border-slate-200 text-left text-[11px] uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-2.5 font-semibold">{t("base.field.name")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("mod.inventory.warehouses.title")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("mod.inventory.replenishment.qty")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("base.field.status")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {low.map((level) => {
                  const quantity = Number(level.quantity);
                  return (
                    <tr key={level.id} className="hover:bg-slate-50/60">
                      <td className="px-4 py-3 font-medium text-slate-900">{products[level.product_id] || level.product_id}</td>
                      <td className="px-4 py-3 text-slate-600">{warehouses[level.warehouse_id] || level.warehouse_id}</td>
                      <td className="px-4 py-3 text-right tabular-nums font-semibold">{quantity}</td>
                      <td className="px-4 py-3">
                        {quantity <= 0
                          ? <Chip tone="rose">{t("mod.inventory.replenishment.out")}</Chip>
                          : <Chip tone="amber">{t("mod.inventory.replenishment.low")}</Chip>}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
    </Screen>
  );
}
