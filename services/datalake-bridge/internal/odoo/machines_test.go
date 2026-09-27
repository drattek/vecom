package odoo

import (
	"encoding/json"
	"testing"
)

func TestM2OUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{`[42, "WH/Stock"]`, 42}, // many2one poblado
		{`false`, 0},             // many2one vacío
		{`7`, 7},                 // id plano
		{`[8]`, 8},               // par sin display_name
		{`null`, 0},              // null
	}
	for _, c := range cases {
		var m M2O
		if err := json.Unmarshal([]byte(c.in), &m); err != nil {
			t.Fatalf("Unmarshal(%s) error: %v", c.in, err)
		}
		if m.ID != c.want {
			t.Errorf("Unmarshal(%s).ID = %d, quiero %d", c.in, m.ID, c.want)
		}
	}
}

func TestWarehouseDecode(t *testing.T) {
	// Forma real de un search_read de stock.warehouse en JSON-2.
	const payload = `[{"id":8,"lot_stock_id":[15,"VLEN/Stock"],"company_id":[2,"VEGUSA RENTAL STORE SA DE CV"]}]`
	var whs []Warehouse
	if err := json.Unmarshal([]byte(payload), &whs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(whs) != 1 {
		t.Fatalf("len = %d", len(whs))
	}
	w := whs[0]
	if w.ID != 8 || w.LotStockID.ID != 15 || w.CompanyID.ID != 2 {
		t.Fatalf("decodificado mal: %+v", w)
	}
}

func TestStrUnmarshal(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"5000 LBS"`, "5000 LBS"}, // texto normal
		{`false`, ""},              // campo vacío en Odoo llega como false
		{`null`, ""},
		{`""`, ""},
	}
	for _, c := range cases {
		var s Str
		if err := json.Unmarshal([]byte(c.in), &s); err != nil {
			t.Fatalf("Unmarshal(%s): %v", c.in, err)
		}
		if string(s) != c.want {
			t.Errorf("Unmarshal(%s)=%q, quiero %q", c.in, string(s), c.want)
		}
	}
}
