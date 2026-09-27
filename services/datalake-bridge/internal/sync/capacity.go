package sync

import (
	"regexp"
	"strings"
)

// La capacidad NO está replicada en el datalake (el atributo CAPACIDAD existe
// pero su valor no viene). Como los nombres de las máquinas suelen traerla
// ("5000 LBS", "2.5 Tons", "2500 KGS"), se extrae de ahí. Es aproximado: para
// plataformas/compresores el nombre trae otra cosa (pies, CFM) y ahí no hay match.
var reCapacity = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(lbs?|toneladas?|tons?|kgs?|kg)\b`)

// capacityFromName devuelve una capacidad normalizada ("5000 LBS", "2.5 Ton",
// "2500 KG") o "" si el nombre no trae una reconocible.
func capacityFromName(name string) string {
	m := reCapacity.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	num := strings.Replace(m[1], ",", ".", 1)
	var unit string
	switch u := strings.ToLower(m[2]); {
	case strings.HasPrefix(u, "lb"):
		unit = "LBS"
	case strings.HasPrefix(u, "ton"):
		unit = "Ton"
	case strings.HasPrefix(u, "kg"):
		unit = "KG"
	}
	return num + " " + unit
}
