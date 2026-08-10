/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Inventory Demand Forecaster & Safety Stock Reorder Analysis.
 */

package ai

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ForecastRecommendation struct {
	ProductID        string `json:"product_id"`
	SKU              string `json:"sku"`
	ProductName      string `json:"product_name"`
	CurrentStock     int64  `json:"current_stock"`
	RecommendedMin   int64  `json:"recommended_min"`
	ReorderAlert     bool   `json:"reorder_alert"`
	SuggestedReorder int64  `json:"suggested_reorder"`
}

type Forecaster struct {
	db *pgxpool.Pool
}

func NewForecaster(db *pgxpool.Pool) *Forecaster {
	return &Forecaster{db: db}
}

func (f *Forecaster) AnalyzeTenantStock(ctx context.Context, tenantID string) ([]ForecastRecommendation, error) {
	rows, err := f.db.Query(ctx,
		`SELECT p.id, p.sku, p.name, COALESCE(SUM(sl.quantity), 0) as total_qty
		 FROM products p
		 LEFT JOIN stock_levels sl ON sl.product_id = p.id AND sl.tenant_id = p.tenant_id
		 WHERE p.tenant_id = $1
		 GROUP BY p.id, p.sku, p.name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// A forecast is only worth acting on if it saw every product. Dropping the
	// rows that failed to scan, and returning early on a broken stream, both
	// produce a reorder list that is short in exactly the direction that hurts:
	// the product nobody was told to reorder.
	list := make([]ForecastRecommendation, 0)
	for rows.Next() {
		var item ForecastRecommendation
		if err := rows.Scan(&item.ProductID, &item.SKU, &item.ProductName, &item.CurrentStock); err != nil {
			return nil, err
		}
		item.RecommendedMin = 10 // Safety stock baseline
		if item.CurrentStock < item.RecommendedMin {
			item.ReorderAlert = true
			item.SuggestedReorder = (item.RecommendedMin * 2) - item.CurrentStock
		}
		list = append(list, item)
	}
	return list, rows.Err()
}
