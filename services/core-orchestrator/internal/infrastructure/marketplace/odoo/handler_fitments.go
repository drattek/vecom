package odoo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

// FitmentsHandler pushes a product's machine/vehicle compatibilities to the
// website_vegusa module through its ecom_sync_fitments method
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
	// Skipped lists the fitments whose sync was skipped because they still
	// collide with an existing machine.model after reusing it (not part of
	// Odoo's response). Nothing was written to Odoo for them.
	Skipped []string `json:"-"`
}

// SyncProductFitments replaces the machine models core-orchestrator manages on
// productTmplID with fitments (the product's complete list). Models loaded by
// hand in Odoo are never removed — see ecom_sync_fitments. productBrand, when
// not nil, is the brand assigned to the product itself: Odoo links it to the
// template even when fitments is empty.
func (h *FitmentsHandler) SyncProductFitments(ctx context.Context, credentials Credentials, productTmplID int64, fitments []Fitment, productBrand *FitmentBrand) (*SyncFitmentsResult, error) {
	if fitments == nil {
		fitments = []Fitment{}
	}

	params := map[string]any{
		"ids":      []int64{productTmplID},
		"fitments": fitments,
	}
	if productBrand != nil {
		params["product_brand"] = productBrand
	}

	result, err := h.callSyncFitments(ctx, credentials, params)
	if err != nil && isFitmentUniqueViolation(err) {
		// Odoo ya tiene un machine.model con la misma marca/tipo/nombre/años pero
		// con OTRO ecom_ref (otro fitment local equivalente): el módulo solo adopta
		// registros sin ecom_ref, así que intenta crear el nuestro y viola la
		// unicidad. Se busca el existente (solo lectura) y se manda su ecom_ref para
		// que el módulo lo vincule en vez de crear uno nuevo.
		resolved, changed, resolveErr := h.reuseExistingModels(ctx, credentials, fitments)
		if resolveErr != nil {
			log.Printf("odoo fitments: product.template %d — could not look up the existing machine.model after a uniqueness error: %v", productTmplID, resolveErr)
		} else if changed {
			params["fitments"] = resolved
			result, err = h.callSyncFitments(ctx, credentials, params)
		}

		// Si aun así sigue chocando no se hace nada más contra Odoo: no se prueban
		// subconjuntos ni se quita ningún vínculo. Las compatibilidades de este
		// producto se omiten (quedan como estaban en Odoo) y el resto de sus datos
		// continúa; se reporta en el log y en el resultado.
		if err != nil && isFitmentUniqueViolation(err) {
			log.Printf("odoo fitments: product.template %d — compatibilities skipped, they still collide with an existing machine.model after reusing it: %v", productTmplID, err)
			skipped := make([]string, 0, len(fitments))
			for _, fitment := range fitments {
				skipped = append(skipped, fmt.Sprintf("%s %d", fitment.Name, fitment.YearStart))
			}
			return &SyncFitmentsResult{Skipped: skipped}, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error syncing fitments of product.template %d: %w", productTmplID, err)
	}

	return result, nil
}

func (h *FitmentsHandler) callSyncFitments(ctx context.Context, credentials Credentials, params map[string]any) (*SyncFitmentsResult, error) {
	var result SyncFitmentsResult
	if err := h.client.Call(ctx, credentials, "product.template", "ecom_sync_fitments", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// isFitmentUniqueViolation reports whether err is the 422 website_vegusa raises
// when two machine.model rows would share brand, type, name and years.
func isFitmentUniqueViolation(err error) bool {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.Status != 422 {
		return false
	}
	return strings.Contains(strings.ToLower(statusErr.Message), "must be unique")
}

type modelFieldInfo struct {
	Type     string `json:"type"`
	Relation string `json:"relation"`
}

type existingModelRow struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	EcomRef any    `json:"ecom_ref"`
}

// fitmentIdentity is what Odoo's uniqueness check compares (name case-insensitive).
func fitmentIdentity(fitment Fitment) string {
	return fmt.Sprintf("%s|%s|%s|%d|%d",
		strings.ToLower(strings.TrimSpace(fitment.Name)),
		strings.ToLower(strings.TrimSpace(fitment.Brand.Name)),
		strings.ToLower(strings.TrimSpace(fitment.Type.Name)),
		fitment.YearStart, fitment.YearEnd)
}

// reuseExistingModels prepares the retry after a uniqueness error. It only READS
// from Odoo (fields_get, search_read): for every fitment it looks up the
// machine.model Odoo already has with the same brand/type/name/years and, when
// that record carries a different ecom_ref, swaps the fitment's ecom_ref (and
// takes its exact name, so the shared record is not renamed) for it. It also
// drops fitments that end up duplicated, by ecom_ref or by identity. changed is
// false when nothing differs from fitments, so a retry would be pointless.
func (h *FitmentsHandler) reuseExistingModels(ctx context.Context, credentials Credentials, fitments []Fitment) ([]Fitment, bool, error) {
	// The brand/type relation field names of machine.model belong to the Odoo
	// module, so they are discovered instead of hard-coded.
	var fields map[string]modelFieldInfo
	if err := h.client.Call(ctx, credentials, "machine.model", "fields_get", map[string]any{"attributes": []string{"type", "relation"}}, &fields); err != nil {
		return nil, false, err
	}
	brandField, typeField := "", ""
	for name, info := range fields {
		if info.Type != "many2one" {
			continue
		}
		switch info.Relation {
		case "machine.brand":
			brandField = name
		case "machine.type":
			typeField = name
		}
	}

	resolved := make([]Fitment, 0, len(fitments))
	for _, fitment := range fitments {
		domain := []any{
			[]any{"name", "=ilike", fitment.Name},
			[]any{"year_start", "=", fitment.YearStart},
			[]any{"year_end", "=", fitment.YearEnd},
		}
		if brandField != "" {
			domain = append(domain, []any{brandField + ".name", "=ilike", fitment.Brand.Name})
		}
		if typeField != "" {
			domain = append(domain, []any{typeField + ".name", "=ilike", fitment.Type.Name})
		}

		var rows []existingModelRow
		params := map[string]any{"domain": domain, "fields": []string{"ecom_ref", "name"}, "limit": 5}
		if err := h.client.Call(ctx, credentials, "machine.model", "search_read", params, &rows); err != nil {
			return nil, false, err
		}

		for _, row := range rows {
			existingRef, ok := row.EcomRef.(string)
			if !ok || existingRef == "" || existingRef == fitment.EcomRef {
				continue
			}
			log.Printf("odoo fitments: %q (%s) already exists in Odoo as machine.model %d with ecom_ref %q; using it instead of %q",
				fitment.Name, fitment.Brand.Name, row.ID, existingRef, fitment.EcomRef)
			fitment.EcomRef = existingRef
			if strings.TrimSpace(row.Name) != "" {
				fitment.Name = row.Name
			}
			break
		}
		resolved = append(resolved, fitment)
	}

	// Two of ours can now point at the same existing record, or were equivalent
	// from the start (the module would try to create both and collide).
	seenRef := make(map[string]bool, len(resolved))
	seenIdentity := make(map[string]bool, len(resolved))
	unique := make([]Fitment, 0, len(resolved))
	for _, fitment := range resolved {
		identity := fitmentIdentity(fitment)
		if seenRef[fitment.EcomRef] || seenIdentity[identity] {
			continue
		}
		seenRef[fitment.EcomRef] = true
		seenIdentity[identity] = true
		unique = append(unique, fitment)
	}

	changed := len(unique) != len(fitments)
	for i := range unique {
		if !changed && (unique[i].EcomRef != fitments[i].EcomRef || unique[i].Name != fitments[i].Name) {
			changed = true
		}
	}
	if !changed {
		return fitments, false, nil
	}

	return unique, true, nil
}
