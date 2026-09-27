package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// M2O decodifica un campo many2one de Odoo, que en JSON-2 llega como
// [id, "display_name"] o como false. Solo interesa el id.
type M2O struct{ ID int64 }

func (m *M2O) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "false" || s == "null" {
		m.ID = 0
		return nil
	}
	var pair []json.RawMessage
	if err := json.Unmarshal(b, &pair); err == nil && len(pair) >= 1 {
		return json.Unmarshal(pair[0], &m.ID)
	}
	return json.Unmarshal(b, &m.ID)
}

// Warehouse es el subconjunto de stock.warehouse que necesitamos para colocar
// existencia: la ubicación de stock y la compañía.
type Warehouse struct {
	ID         int64 `json:"id"`
	Code       string
	LotStockID M2O `json:"lot_stock_id"`
	CompanyID  M2O `json:"company_id"`
}

// ResolveWarehouse busca un almacén por su código y devuelve su ubicación de
// stock y compañía. found=false si no existe (el sync lo trata como error de
// mapeo, no crea almacenes).
func (c *Client) ResolveWarehouse(ctx context.Context, code string) (Warehouse, bool, error) {
	var res []Warehouse
	err := c.Call(ctx, "stock.warehouse", "search_read", map[string]any{
		"domain": [][]any{{"code", "=", code}},
		"fields": []string{"id", "lot_stock_id", "company_id"},
		"limit":  1,
	}, &res)
	if err != nil {
		return Warehouse{}, false, err
	}
	if len(res) == 0 {
		return Warehouse{}, false, nil
	}
	res[0].Code = code
	return res[0], true, nil
}

// EnsureCategory devuelve el id de la categoría de producto con ese nombre,
// creándola si no existe.
func (c *Client) EnsureCategory(ctx context.Context, name string) (int64, error) {
	var found []struct {
		ID int64 `json:"id"`
	}
	if err := c.Call(ctx, "product.category", "search_read", map[string]any{
		"domain": [][]any{{"name", "=", name}},
		"fields": []string{"id"},
		"limit":  1,
	}, &found); err != nil {
		return 0, err
	}
	if len(found) > 0 {
		return found[0].ID, nil
	}
	var ids []int64
	if err := c.Call(ctx, "product.category", "create", map[string]any{
		"vals_list": []map[string]any{{"name": name}},
	}, &ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("product.category.create no devolvió id")
	}
	return ids[0], nil
}

// Product es el producto (variante) resuelto por su default_code.
type Product struct {
	ID          int64  `json:"id"`
	Tmpl        M2O    `json:"product_tmpl_id"`
	DefaultCode string `json:"default_code"`
	Tracking    string `json:"tracking"`
}

// FindProduct busca la variante (product.product) por default_code.
func (c *Client) FindProduct(ctx context.Context, code string) (Product, bool, error) {
	var res []Product
	err := c.Call(ctx, "product.product", "search_read", map[string]any{
		"domain": [][]any{{"default_code", "=", code}},
		"fields": []string{"id", "product_tmpl_id", "default_code", "tracking"},
		"limit":  1,
	}, &res)
	if err != nil {
		return Product{}, false, err
	}
	if len(res) == 0 {
		return Product{}, false, nil
	}
	return res[0], true, nil
}

// CreateMachineProduct crea un product.template de máquina (almacenable, con
// seguimiento por número de serie, cotizable aunque esté en 0) y devuelve el id
// de su variante (product.product). Odoo crea automáticamente una variante para
// una plantilla sin atributos.
func (c *Client) CreateMachineProduct(ctx context.Context, code, name string, categoryID int64) (Product, error) {
	vals := map[string]any{
		"name":         name,
		"default_code": code,
		"type":         "consu", // en Odoo 19 el almacenable es consu + is_storable
		"is_storable":  true,
		"tracking":     "serial",
		"categ_id":     categoryID,
		"sale_ok":      true,
		"purchase_ok":  true,
	}
	var ids []int64
	if err := c.Call(ctx, "product.template", "create", map[string]any{
		"vals_list": []map[string]any{vals},
	}, &ids); err != nil {
		return Product{}, err
	}
	if len(ids) == 0 {
		return Product{}, fmt.Errorf("product.template.create no devolvió id")
	}
	// Recuperar la variante creada para la plantilla.
	p, found, err := c.FindProduct(ctx, code)
	if err != nil {
		return Product{}, err
	}
	if !found {
		return Product{}, fmt.Errorf("no se encontró la variante recién creada para %q", code)
	}
	return p, nil
}

// Lot es un stock.lot (número de serie) de un producto.
type Lot struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// FindLot busca un lote/serie por nombre para un producto.
func (c *Client) FindLot(ctx context.Context, productID int64, serial string) (Lot, bool, error) {
	var res []Lot
	err := c.Call(ctx, "stock.lot", "search_read", map[string]any{
		"domain": [][]any{{"product_id", "=", productID}, {"name", "=", serial}},
		"fields": []string{"id", "name"},
		"limit":  1,
	}, &res)
	if err != nil {
		return Lot{}, false, err
	}
	if len(res) == 0 {
		return Lot{}, false, nil
	}
	return res[0], true, nil
}

// CreateLot crea un stock.lot (serie) para un producto en una compañía.
func (c *Client) CreateLot(ctx context.Context, productID, companyID int64, serial string) (Lot, error) {
	vals := map[string]any{
		"name":       serial,
		"product_id": productID,
	}
	if companyID != 0 {
		vals["company_id"] = companyID
	}
	var ids []int64
	if err := c.Call(ctx, "stock.lot", "create", map[string]any{
		"vals_list": []map[string]any{vals},
	}, &ids); err != nil {
		return Lot{}, err
	}
	if len(ids) == 0 {
		return Lot{}, fmt.Errorf("stock.lot.create no devolvió id")
	}
	return Lot{ID: ids[0], Name: serial}, nil
}

// Quant es una existencia física (stock.quant) de un lote en una ubicación.
type Quant struct {
	ID         int64   `json:"id"`
	ProductID  M2O     `json:"product_id"`
	LotID      M2O     `json:"lot_id"`
	LocationID M2O     `json:"location_id"`
	Quantity   float64 `json:"quantity"`
}

// SerialQuants devuelve las existencias de productos con seguimiento por serie
// (lot_id != false) con cantidad > 0 en las ubicaciones dadas. Es el estado
// actual de máquinas en Odoo, base del diff. Solo trae máquinas: las
// refacciones/accesorios no llevan lote, así que quedan fuera del filtro.
func (c *Client) SerialQuants(ctx context.Context, locationIDs []int64) ([]Quant, error) {
	var res []Quant
	err := c.Call(ctx, "stock.quant", "search_read", map[string]any{
		"domain": []any{
			[]any{"location_id", "in", locationIDs},
			[]any{"lot_id", "!=", false},
			[]any{"quantity", ">", 0},
		},
		"fields": []string{"id", "product_id", "lot_id", "location_id", "quantity"},
	}, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// SetInventory fija la existencia de (producto, lote) en una ubicación a qty,
// vía ajuste de inventario: escribe inventory_quantity con inventory_mode=True
// sobre el quant (creándolo si no existe) y aplica el ajuste. Es el mismo
// patrón que usa el resto del ecosistema Odoo de Vegusa (stock.quant +
// inventory_mode + apply).
func (c *Client) SetInventory(ctx context.Context, productID, lotID, locationID int64, qty float64) error {
	invCtx := map[string]any{"inventory_mode": true}

	// ¿Ya hay un quant para (producto, lote, ubicación)?
	var existing []struct {
		ID int64 `json:"id"`
	}
	if err := c.Call(ctx, "stock.quant", "search_read", map[string]any{
		"domain": [][]any{
			{"product_id", "=", productID},
			{"lot_id", "=", lotID},
			{"location_id", "=", locationID},
		},
		"fields":  []string{"id"},
		"limit":   1,
		"context": invCtx,
	}, &existing); err != nil {
		return err
	}

	var quantID int64
	if len(existing) > 0 {
		quantID = existing[0].ID
		if err := c.Call(ctx, "stock.quant", "write", map[string]any{
			"ids":     []int64{quantID},
			"vals":    map[string]any{"inventory_quantity": qty},
			"context": invCtx,
		}, nil); err != nil {
			return fmt.Errorf("write inventory_quantity: %w", err)
		}
	} else {
		var ids []int64
		if err := c.Call(ctx, "stock.quant", "create", map[string]any{
			"vals_list": []map[string]any{{
				"product_id":         productID,
				"lot_id":             lotID,
				"location_id":        locationID,
				"inventory_quantity": qty,
			}},
			"context": invCtx,
		}, &ids); err != nil {
			return fmt.Errorf("create stock.quant: %w", err)
		}
		if len(ids) == 0 {
			return fmt.Errorf("stock.quant.create no devolvió id")
		}
		quantID = ids[0]
	}

	// Aplicar el ajuste (mueve la existencia contada a la real).
	if err := c.Call(ctx, "stock.quant", "action_apply_inventory", map[string]any{
		"ids":     []int64{quantID},
		"context": invCtx,
	}, nil); err != nil {
		return fmt.Errorf("action_apply_inventory: %w", err)
	}
	return nil
}
