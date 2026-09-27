// Package brand resuelve la marca de una máquina hacia el nombre de product.brand
// en Odoo. La marca en D365 vive en dos lados y ninguno es limpio del todo:
//   - inventtable.brandcodeid_mx: nombre legible pero inconsistente y con huecos
//   - dim. financiera "Marca": mejor cobertura pero en código (UCA/BOB/JLG...)
//
// Se prefiere el nombre legible normalizado; si no hay, se mapea el código.
package brand

import "strings"

// byBrandCode normaliza el valor de brandcodeid_mx hacia el nombre exacto de un
// product.brand de Odoo. "" = desconocido (cae al código de la dim. financiera).
var byBrandCode = map[string]string{
	"BOBCAT":      "Bobcat",
	"BOBCAT MH":   "Bobcat MH",
	"HELI":        "Heli",
	"JLG":         "JLG",
	"FLE":         "Flexi",
	"FLEXI":       "Flexi",
	"UNICARRIER":  "Unicarriers Forklift",
	"UNICARRIERS": "Unicarriers Forklift",
	"NISSAN":      "Nissan",
	"DONGFENG":    "Dongfeng",
}

// byFinDimCode mapea el código de la dimensión financiera Marca al nombre de
// product.brand. OTR ("Otros") es un cajón mixto -> "Otras". Códigos con muy
// pocos productos y sin nombre conocido se dejan sin resolver ("").
var byFinDimCode = map[string]string{
	"UCA": "Unicarriers Forklift",
	"BOB": "Bobcat",
	"MCD": "Bobcat MH",
	"HEL": "Heli",
	"JLG": "JLG",
	"FLE": "Flexi",
	"TDN": "Taylor-Dunn",
	"TEC": "Tecnacar",
	"OTR": "Otras",
}

// Resolve devuelve el nombre de product.brand para una máquina, o "" si no se
// puede determinar (en cuyo caso el sync no toca la marca).
func Resolve(brandCode, finDimCode string) string {
	if n := byBrandCode[strings.ToUpper(strings.TrimSpace(brandCode))]; n != "" {
		return n
	}
	if n := byFinDimCode[strings.ToUpper(strings.TrimSpace(finDimCode))]; n != "" {
		return n
	}
	return ""
}
