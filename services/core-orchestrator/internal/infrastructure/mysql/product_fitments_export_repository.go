package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ProductVehicleFitmentExportDTO is one distinct vehicle fitment a product is
// compatible with, with the brand resolved to its name. The
// motor/position/side qualifiers of ecom_product_vehicle_compatibility are
// deliberately not part of it: they describe how the product occupies the
// fitment (a MercadoLibre concept) and Odoo does not model them.
type ProductVehicleFitmentExportDTO struct {
	FitmentID int64
	BrandID   int64
	BrandName string
	Model     string
	YearStart int
	YearEnd   *int
}

// ProductEquipmentFitmentExportDTO is one equipment fitment a product is
// compatible with, with brand and equipment type resolved to their names.
type ProductEquipmentFitmentExportDTO struct {
	FitmentID         int64
	BrandID           int64
	BrandName         string
	EquipmentTypeID   int64
	EquipmentTypeName string
	Model             string
	Serie             string
}

// ProductFitmentsExportRepository reads the compatibilities of a product in
// the shape channels that publish them need (names instead of foreign keys,
// one row per fitment). Read-only.
type ProductFitmentsExportRepository struct {
	db Querier
}

func NewProductFitmentsExportRepository(db Querier) *ProductFitmentsExportRepository {
	return &ProductFitmentsExportRepository{db: db}
}

// FindVehicleFitmentsByProductID returns the distinct, non-deleted vehicle
// fitments linked to productID (several compatibility rows can point at the
// same fitment with different motor/position/side).
func (r *ProductFitmentsExportRepository) FindVehicleFitmentsByProductID(ctx context.Context, productID int64) ([]ProductVehicleFitmentExportDTO, error) {
	query := `
		SELECT DISTINCT vf.id, vf.brand_id, b.name, vf.model, vf.year_start, vf.year_end
		FROM ecom_product_vehicle_compatibility pvc
		JOIN ecom_vehicle_fitments vf ON vf.id = pvc.vehicle_fitment_id AND vf.deleted_at IS NULL
		JOIN ecom_brands b ON b.id = vf.brand_id AND b.deleted_at IS NULL
		WHERE pvc.product_id = ? AND pvc.deleted_at IS NULL
		ORDER BY vf.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying vehicle fitments of product %d: %w", productID, err)
	}
	defer rows.Close()

	fitments := make([]ProductVehicleFitmentExportDTO, 0)
	for rows.Next() {
		var fitment ProductVehicleFitmentExportDTO
		var yearEnd sql.NullInt64
		if err := rows.Scan(&fitment.FitmentID, &fitment.BrandID, &fitment.BrandName, &fitment.Model, &fitment.YearStart, &yearEnd); err != nil {
			return nil, fmt.Errorf("error scanning vehicle fitment of product %d: %w", productID, err)
		}
		if yearEnd.Valid {
			value := int(yearEnd.Int64)
			fitment.YearEnd = &value
		}
		fitments = append(fitments, fitment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vehicle fitments of product %d: %w", productID, err)
	}

	return fitments, nil
}

// FindEquipmentFitmentsByProductID returns the non-deleted equipment fitments
// linked to productID.
func (r *ProductFitmentsExportRepository) FindEquipmentFitmentsByProductID(ctx context.Context, productID int64) ([]ProductEquipmentFitmentExportDTO, error) {
	query := `
		SELECT ef.id, ef.brand_id, b.name, ef.equipment_type, et.name, ef.model, ef.serie
		FROM ecom_product_equipment_compatibility pec
		JOIN ecom_equipment_fitment ef ON ef.id = pec.equipment_fitment_id AND ef.deleted_at IS NULL
		JOIN ecom_brands b ON b.id = ef.brand_id AND b.deleted_at IS NULL
		JOIN ecom_equipment_types et ON et.id = ef.equipment_type AND et.deleted_at IS NULL
		WHERE pec.product_id = ? AND pec.deleted_at IS NULL
		ORDER BY ef.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying equipment fitments of product %d: %w", productID, err)
	}
	defer rows.Close()

	fitments := make([]ProductEquipmentFitmentExportDTO, 0)
	for rows.Next() {
		var fitment ProductEquipmentFitmentExportDTO
		var model, serie sql.NullString
		if err := rows.Scan(&fitment.FitmentID, &fitment.BrandID, &fitment.BrandName, &fitment.EquipmentTypeID, &fitment.EquipmentTypeName, &model, &serie); err != nil {
			return nil, fmt.Errorf("error scanning equipment fitment of product %d: %w", productID, err)
		}
		fitment.Model = strings.TrimSpace(model.String)
		fitment.Serie = strings.TrimSpace(serie.String)
		fitments = append(fitments, fitment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating equipment fitments of product %d: %w", productID, err)
	}

	return fitments, nil
}
