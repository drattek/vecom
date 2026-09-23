package odoo

import (
	"context"
	"fmt"
)

// FitmentsHandler pushes a product's machine/vehicle compatibilities to the
// website_sale_machine_catalog module through its ecom_sync_fitments method
// on product.template (Odoo's External JSON-2 API). See ADR 0006.
type FitmentsHandler struct {
	client *Client
}

func NewFitmentsHandler(client *Client) *FitmentsHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &FitmentsHandler{client: client}
}

// FitmentBrand identifies a brand in machine.brand. EcomRef is
// "brand:<ecom_brands.id>".
type FitmentBrand struct {
	EcomRef string `json:"ecom_ref"`
	Name    string `json:"name"`
}

// FitmentType identifies a machine type in machine.type. EcomRef is
// "equipment_type:<ecom_equipment_types.id>" for equipment; the vehicle type
// has no ecom_ref (vehicles have no type table) and is flagged IsVehicle.
type FitmentType struct {
	EcomRef   string `json:"ecom_ref,omitempty"`
	Name      string `json:"name"`
	IsVehicle bool   `json:"is_vehicle"`
}

// Fitment is one machine.model row: an equipment fitment (EcomRef
// "equipment_fitment:<id>", no years) or a vehicle fitment (EcomRef
// "vehicle_fitment:<id>", YearStart set, YearEnd 0 when open-ended — Odoo
// stores 0 for "not set").
type Fitment struct {
	EcomRef   string       `json:"ecom_ref"`
	Name      string       `json:"name"`
	YearStart int          `json:"year_start"`
	YearEnd   int          `json:"year_end"`
	Brand     FitmentBrand `json:"brand"`
	Type      FitmentType  `json:"type"`
}

// SyncFitmentsResult mirrors the summary product.template.ecom_sync_fitments
// returns.
type SyncFitmentsResult struct {
	Linked   int `json:"linked"`
	Unlinked int `json:"unlinked"`
}

// SyncProductFitments replaces the machine models core-orchestrator manages on
// productTmplID with fitments (the product's complete list). Models loaded by
// hand in Odoo are never removed — see ecom_sync_fitments.
func (h *FitmentsHandler) SyncProductFitments(ctx context.Context, credentials Credentials, productTmplID int64, fitments []Fitment) (*SyncFitmentsResult, error) {
	if fitments == nil {
		fitments = []Fitment{}
	}

	params := map[string]any{
		"ids":      []int64{productTmplID},
		"fitments": fitments,
	}

	var result SyncFitmentsResult
	if err := h.client.Call(ctx, credentials, "product.template", "ecom_sync_fitments", params, &result); err != nil {
		return nil, fmt.Errorf("error syncing fitments of product.template %d: %w", productTmplID, err)
	}

	return &result, nil
}
