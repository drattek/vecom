package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	// summaryFirstWindow es lo que cubre el primer resumen.
	summaryFirstWindow = 24 * time.Hour
	// summaryMaxWindow limita lo que se recupera tras estar apagado.
	summaryMaxWindow = 72 * time.Hour

	zeroStockReason = "zero_stock"
)

type stockPauseSource interface {
	// StockPausedListings devuelve las publicaciones no cerradas de los
	// productos que tuvieron una pausa en los últimos window y siguen con alguna
	// pausada por falta de stock.
	StockPausedListings(ctx context.Context, window time.Duration, marketplacePauseMessage string) ([]mysqlInfra.ProductListingDTO, error)
}

type pausedProduct struct {
	sku, name string
	// paused: publicaciones pausadas por stock, por conexión y en orden.
	connections []string
	paused      map[string][]string
	// stillActive: conexiones donde el producto sigue publicado y activo.
	stillActive []string
	rows        [][]string
}

func groupPausedProducts(listings []mysqlInfra.ProductListingDTO) []*pausedProduct {
	byID := map[int64]*pausedProduct{}
	ordered := make([]*pausedProduct, 0)
	for _, listing := range listings {
		product := byID[listing.ProductID]
		if product == nil {
			product = &pausedProduct{sku: listing.SKU, name: listing.ProductName, paused: map[string][]string{}}
			byID[listing.ProductID] = product
			ordered = append(ordered, product)
		}

		// En MercadoLibre se identifica por su id; en otros canales, por título.
		label := listing.ExternalID
		if !strings.HasPrefix(label, "ML") && listing.Title != "" {
			label = listing.Title
		}
		if label == "" {
			label = "sin identificador"
		}

		switch {
		case listing.Status == "paused" && listing.PausedReason == zeroStockReason:
			if _, seen := product.paused[listing.ConnectionName]; !seen {
				product.connections = append(product.connections, listing.ConnectionName)
			}
			product.paused[listing.ConnectionName] = appendUnique(product.paused[listing.ConnectionName], label)
			product.rows = append(product.rows, []string{product.sku, product.name, listing.ConnectionName, label, "pausada sin stock"})
		case listing.Status == "synced":
			product.stillActive = appendUnique(product.stillActive, listing.ConnectionName)
			product.rows = append(product.rows, []string{product.sku, product.name, listing.ConnectionName, label, "activa"})
		}
	}

	paused := ordered[:0]
	for _, product := range ordered {
		if len(product.connections) > 0 {
			paused = append(paused, product)
		}
	}
	return paused
}

func (p *pausedProduct) block() string {
	var text strings.Builder
	fmt.Fprintf(&text, "%s · %s\nPausado por falta de stock en:", p.sku, p.name)
	for _, connection := range p.connections {
		fmt.Fprintf(&text, "\n • %s: %s", connection, strings.Join(p.paused[connection], ", "))
	}
	if len(p.stillActive) > 0 {
		fmt.Fprintf(&text, "\n Sigue activo en: %s", strings.Join(p.stillActive, ", "))
	}
	return text.String()
}

// stockSummaryNotice arma un renglón por producto. Si no cabe en un mensaje, el
// detalle va en un archivo.
func stockSummaryNotice(products []*pausedProduct) notice {
	header := fmt.Sprintf("📦 Productos sin stock con publicaciones pausadas: %d", len(products))
	n := notice{notificationType: TypeStockPauseSummary}

	blocks := make([]string, 0, len(products))
	rows := make([][]string, 0, len(products))
	for _, product := range products {
		blocks = append(blocks, product.block())
		rows = append(rows, product.rows...)
	}
	if full := header + "\n\n" + strings.Join(blocks, "\n\n"); utf8.RuneCountInString(full) <= maxMessageRunes {
		n.text = full
		return n
	}

	n.text = header + "\n\nSon demasiados para un mensaje: el detalle va en el archivo."
	n.attachmentName = "sin-stock.csv"
	n.attachment = buildCSV([]string{"sku", "producto", "conexion", "publicacion", "estado"}, rows)
	return n
}

// sendStockPauseSummary informa, una sola vez, de las pausas ocurridas desde el
// resumen anterior que sigan vigentes.
func (s *Service) sendStockPauseSummary(ctx context.Context) (jobWindow, error) {
	to := s.now()
	from := to.Add(-summaryFirstWindow)
	previous, err := s.jobs.LastWindowEnd(ctx, jobStockPauseSummary)
	if err != nil {
		return jobWindow{}, err
	}
	if previous != nil {
		from = *previous
	}
	if oldest := to.Add(-summaryMaxWindow); from.Before(oldest) {
		from = oldest
	}
	window := jobWindow{from: &from, to: &to}

	listings, err := s.stockPauses.StockPausedListings(ctx, to.Sub(from), channelListingsApp.MarketplaceOutOfStockPauseMessage)
	if err != nil {
		return jobWindow{}, err
	}
	products := groupPausedProducts(listings)
	if len(products) == 0 {
		return window, nil
	}

	n := stockSummaryNotice(products)
	err = s.inUnit(ctx, func(tx unit) error {
		return s.enqueue(ctx, tx, fmt.Sprintf("stock_summary:%d", to.Unix()), n)
	})
	return window, err
}
