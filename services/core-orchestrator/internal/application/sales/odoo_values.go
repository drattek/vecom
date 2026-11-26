package sales

import (
	"strconv"
	"strings"
	"time"

	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
)

// Conversión de valores de la API de Odoo: un campo vacío llega como false.

const odooTimeLayout = "2006-01-02 15:04:05"

func odooString(record odooInfra.Record, field string) string {
	value, _ := record[field].(string)
	return strings.TrimSpace(value)
}

func odooFloat(record odooInfra.Record, field string) float64 {
	value, _ := record[field].(float64)
	return value
}

func odooBool(record odooInfra.Record, field string) bool {
	value, _ := record[field].(bool)
	return value
}

func odooInt(value any) int64 {
	number, _ := value.(float64)
	return int64(number)
}

// odooTime interpreta una fecha de Odoo, que siempre está en UTC.
func odooTime(record odooInfra.Record, field string) *time.Time {
	text := odooString(record, field)
	if text == "" {
		return nil
	}
	parsed, err := time.ParseInLocation(odooTimeLayout, text, time.UTC)
	if err != nil {
		return nil
	}
	return &parsed
}

// odooRef lee un many2one [id, "nombre"].
func odooRef(record odooInfra.Record, field string) (int64, string) {
	pair, ok := record[field].([]any)
	if !ok || len(pair) == 0 {
		return odooInt(record[field]), ""
	}
	name := ""
	if len(pair) > 1 {
		name, _ = pair[1].(string)
	}
	return odooInt(pair[0]), name
}

// odooIDs lee un one2many/many2many.
func odooIDs(record odooInfra.Record, field string) []int64 {
	values, _ := record[field].([]any)
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		if id := odooInt(value); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func odooID(record odooInfra.Record) int64 { return odooInt(record["id"]) }

func odooIDString(id int64) string {
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}
