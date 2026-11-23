package mysql

import (
	"errors"
	"strings"
	"unicode"
)

// ErrInvalidLocationName se devuelve al intentar crear o renombrar una sucursal o un
// almacén con un nombre sin ninguna letra ni dígito (vacío, solo espacios, caracteres
// invisibles). Esos nombres crearon sucursales "Sin sucursal" y almacenes duplicados.
var ErrInvalidLocationName = errors.New("branch/warehouse name must contain at least one letter or digit")

// NormalizeLocationName quita los espacios (U+0020) del inicio y del final del nombre de
// una sucursal o almacén. Es lo mismo que hace TRIM() en MySQL (y que usan el script de
// normalización y los CHECK de la base), así que lo que busca el código coincide con lo
// que hay guardado. Solo toca esos espacios: no cambia mayúsculas ni espacios internos.
func NormalizeLocationName(name string) string {
	return strings.Trim(name, " ")
}

// HasVisibleName dice si name tiene al menos una letra o un dígito. Es el mismo criterio
// que el CHECK de ecom_branches (REGEXP '[[:alnum:]]'), así que app y base de datos
// aceptan y rechazan lo mismo.
func HasVisibleName(name string) bool {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
