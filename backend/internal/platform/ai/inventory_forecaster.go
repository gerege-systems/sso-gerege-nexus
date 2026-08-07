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

	var list []ForecastRecommendation
	for rows.Next() {
		var item ForecastRecommendation
		if err := rows.Scan(&item.ProductID, &item.SKU, &item.ProductName, &item.CurrentStock); err == nil {
			item.RecommendedMin = 10 // Safety stock baseline
			if item.CurrentStock < item.RecommendedMin {
				item.ReorderAlert = true
				item.SuggestedReorder = (item.RecommendedMin * 2) - item.CurrentStock
			}
			list = append(list, item)
		}
	}
	return list, nil
}
