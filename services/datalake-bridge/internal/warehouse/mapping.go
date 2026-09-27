// Package warehouse mapea la sucursal de D365 (empresa + InventSiteId) al código
// de almacén de Odoo. Los almacenes se crearon una sola vez con estos códigos
// (ver reference-vecom-odoo-warehouse-mapping); aquí solo se traduce D365 -> Odoo
// y el sync resuelve el código contra Odoo en caliente para no fijar ids.
package warehouse

import (
	"fmt"
	"strings"
)

// codeByCompanySite: empresa (dataareaid) -> sucursal (InventSiteId) -> código Odoo.
// La única sucursal que no coincide con su propio nombre es LEN, que en msb es el
// almacén preexistente "WH" y en vrs es "VLEN".
var codeByCompanySite = map[string]map[string]string{
	"msb": {
		"LEN": "WH",
		"AGS": "AGS",
		"GDL": "GDL",
		"IRP": "IRP",
		"MEX": "MEX",
		"QRO": "QRO",
		"SLP": "SLP",
	},
	"vrs": {
		"LEN": "VLEN",
	},
}

// OdooCode devuelve el código de almacén Odoo para una empresa+sucursal de D365.
func OdooCode(company, site string) (string, error) {
	company = strings.ToLower(strings.TrimSpace(company))
	site = strings.ToUpper(strings.TrimSpace(site))
	sites, ok := codeByCompanySite[company]
	if !ok {
		return "", fmt.Errorf("empresa D365 sin mapeo a Odoo: %q", company)
	}
	code, ok := sites[site]
	if !ok {
		return "", fmt.Errorf("sucursal %q de la empresa %q sin almacén mapeado en Odoo", site, company)
	}
	return code, nil
}
