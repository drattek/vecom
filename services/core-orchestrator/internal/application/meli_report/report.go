package meli_report

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
)

const (
	// tokenMargin: un token a punto de vencer no se usa.
	tokenMargin = time.Minute
	attempts    = 3
)

// ItemSource lo implementa mercadolibre.ItemsHandler.
type ItemSource interface {
	GetItemsStatus(ctx context.Context, accessToken string, externalIDs []string) ([]mercadoLibreInfra.ItemStatus, error)
}

// TokenSource devuelve el token guardado de una conexión, sin renovarlo.
type TokenSource func(ctx context.Context, connectionID int64) (token string, expiresAt time.Time, err error)

type Runner struct {
	Items ItemSource
	Token TokenSource
	// Fold traduce el estado de MercadoLibre al vocabulario local.
	Fold func(rawStatus string, subStatus []string) string
	// Progress recibe mensajes de avance; puede ser nil.
	Progress   func(string)
	RetryDelay time.Duration
	Now        func() time.Time
}

// Row es una publicación con al menos una diferencia.
type Row struct {
	Listing  Listing
	Live     Live
	Findings []Finding
}

type Result struct {
	Compared int
	// NotQueried son las que no se pudieron consultar; si no es 0, el reporte
	// está incompleto.
	NotQueried int
	Rows       []Row
	// Problems explica por qué faltó consultar.
	Problems []string
}

func (r *Runner) progress(format string, args ...any) {
	if r.Progress != nil {
		r.Progress(fmt.Sprintf(format, args...))
	}
}

// Run consulta MercadoLibre y compara. Un fallo de una conexión o de un lote no
// detiene el resto: esas publicaciones quedan como no consultadas.
func (r *Runner) Run(ctx context.Context, listings []Listing) (*Result, error) {
	byConnection := map[int64][]Listing{}
	order := make([]int64, 0)
	for _, listing := range listings {
		if _, seen := byConnection[listing.ConnectionID]; !seen {
			order = append(order, listing.ConnectionID)
		}
		byConnection[listing.ConnectionID] = append(byConnection[listing.ConnectionID], listing)
	}

	result := &Result{}
	for _, connectionID := range order {
		group := byConnection[connectionID]
		live, problem := r.fetch(ctx, connectionID, group)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if problem != nil {
			result.Problems = append(result.Problems, fmt.Sprintf("%s: %v", group[0].ConnectionName, problem))
		}
		for _, listing := range group {
			state := live[listing.ExternalID]
			result.Compared++
			if !state.Queried {
				result.NotQueried++
			}
			if findings := Compare(listing, state); len(findings) > 0 {
				result.Rows = append(result.Rows, Row{Listing: listing, Live: state, Findings: findings})
			}
		}
	}

	sort.SliceStable(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if a.Findings[0].Severity != b.Findings[0].Severity {
			return a.Findings[0].Severity < b.Findings[0].Severity
		}
		return a.Listing.SKU < b.Listing.SKU
	})
	return result, nil
}

// fetch consulta por lotes las publicaciones de una conexión.
func (r *Runner) fetch(ctx context.Context, connectionID int64, listings []Listing) (map[string]Live, error) {
	ids := make([]string, 0, len(listings))
	seen := map[string]bool{}
	for _, listing := range listings {
		if !seen[listing.ExternalID] {
			seen[listing.ExternalID] = true
			ids = append(ids, listing.ExternalID)
		}
	}

	live := make(map[string]Live, len(ids))
	var failed error
	for start := 0; start < len(ids); start += mercadoLibreInfra.MaxItemsBatchSize {
		batch := ids[start:min(start+mercadoLibreInfra.MaxItemsBatchSize, len(ids))]

		// Se vuelve a leer en cada lote: el servidor puede haberlo renovado.
		token, expiresAt, err := r.Token(ctx, connectionID)
		if err != nil {
			return live, fmt.Errorf("no se pudo leer el token: %w", err)
		}
		if !expiresAt.After(r.Now().Add(tokenMargin)) {
			return live, errors.New("el token guardado está vencido; el reporte no lo renueva. Con core-orchestrator en marcha, espera su renovación y repite")
		}

		var statuses []mercadoLibreInfra.ItemStatus
		for attempt := 1; ; attempt++ {
			statuses, err = r.Items.GetItemsStatus(ctx, token, batch)
			if err == nil || attempt == attempts || ctx.Err() != nil {
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(r.RetryDelay):
			}
		}
		if err != nil {
			if failed == nil {
				failed = fmt.Errorf("falló al menos un lote: %w", err)
			}
			continue
		}

		for _, id := range batch {
			live[id] = Live{Queried: true}
		}
		for _, status := range statuses {
			live[status.ID] = Live{
				Queried: true, Found: true,
				Status: r.Fold(status.Status, status.SubStatus), RawStatus: status.Status, SubStatus: status.SubStatus,
				Quantity: status.AvailableQuantity, Price: status.Price, LogisticType: status.LogisticType,
			}
		}
		r.progress("%s: %d/%d consultadas", listings[0].ConnectionName, min(start+len(batch), len(ids)), len(ids))
	}
	return live, failed
}

var csvHeader = []string{
	"prioridad", "conexion", "sku", "producto", "publicacion", "problema", "accion_sugerida", "otras_diferencias",
	"estado_local", "motivo_pausa_local", "estado_mercadolibre", "subestado_mercadolibre",
	"stock_local", "cantidad_mercadolibre", "precio_calculado", "precio_mercadolibre", "nota_precio",
	"ultimo_envio", "ultimo_cambio_stock",
}

// WriteCSV escribe un renglón por publicación con diferencias, de lo más urgente
// a lo menos. Lleva BOM para Excel.
func WriteCSV(out io.Writer, rows []Row) error {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	writer.UseCRLF = true
	if err := writer.Write(csvHeader); err != nil {
		return err
	}

	for _, row := range rows {
		listing, live, first := row.Listing, row.Live, row.Findings[0]
		others := make([]string, 0, len(row.Findings)-1)
		for _, finding := range row.Findings[1:] {
			others = append(others, finding.Problem)
		}
		quantity := ""
		if live.Quantity != nil {
			quantity = strconv.Itoa(*live.Quantity)
		}
		cells := []string{
			first.Severity.String(), listing.ConnectionName, listing.SKU, listing.ProductName, listing.ExternalID,
			first.Problem, first.Action, strings.Join(others, " | "),
			listing.Status, listing.PausedReason, live.RawStatus, strings.Join(live.SubStatus, ", "),
			strconv.Itoa(listing.Stock), quantity, formatPrice(listing.ExpectedPrice), formatPrice(live.Price), listing.PriceNote,
			formatTime(listing.LastPushAt), formatTime(listing.StockChangedAt),
		}
		for i, cell := range cells {
			// Excel no debe interpretar ninguna celda como fórmula.
			if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
				cells[i] = "'" + cell
			}
		}
		if err := writer.Write(cells); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	_, err := out.Write(buffer.Bytes())
	return err
}

func formatPrice(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', 2, 64)
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}
