package sales

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ErrInvalidSalesFilter indica un filtro con un valor no admitido.
var ErrInvalidSalesFilter = errors.New("invalid sales filter")

const (
	defaultPageSize = 25
	maxPageSize     = 100
	maxSearchLength = 100
)

var (
	orderStatuses   = []string{OrderPending, OrderConfirmed, OrderCancelled, StatusUnknown}
	paymentStatuses = []string{PaymentPending, PaymentPaid, PaymentPartiallyRefunded, PaymentRefunded, PaymentRejected, StatusUnknown}
	shipStatuses    = []string{ShippingPending, ShippingReadyToShip, ShippingShipped, ShippingDelivered, ShippingNotDelivered, ShippingCancelled, ShippingNotApplicable, StatusUnknown}
)

// ListInput son los filtros tal como llegan de la API. From y To son fechas
// AAAA-MM-DD, ambas incluidas.
type ListInput struct {
	ChannelCode    string
	ConnectionID   int64
	OrderStatus    string
	PaymentStatus  string
	ShippingStatus string
	From           string
	To             string
	Search         string
	Offset         int
	PageSize       int
}

type ListResult struct {
	Total    int64                        `json:"total"`
	Offset   int                          `json:"offset"`
	PageSize int                          `json:"pageSize"`
	Sales    []mysqlInfra.SaleListItemDTO `json:"sales"`
}

type Detail struct {
	mysqlInfra.SaleDetailDTO
	// RelatedSales son las otras órdenes de la misma compra.
	RelatedSales []mysqlInfra.RelatedSaleDTO `json:"relatedSales"`
	// EstimatedNetAmount es total menos comisión y envío del vendedor. nil
	// cuando falta algún dato; no es una conciliación.
	EstimatedNetAmount *float64 `json:"estimatedNetAmount"`
	// ShippingCostOnRelatedSale: el envío es compartido y su costo se restó en
	// otra orden del paquete.
	ShippingCostOnRelatedSale bool `json:"shippingCostOnRelatedSale"`
}

type QueryService struct {
	repository *mysqlInfra.SalesViewRepository
}

func NewQueryService(repository *mysqlInfra.SalesViewRepository) *QueryService {
	return &QueryService{repository: repository}
}

func (s *QueryService) List(ctx context.Context, input ListInput) (*ListResult, error) {
	filter, err := buildFilter(input)
	if err != nil {
		return nil, err
	}
	sales, total, err := s.repository.FindPaginated(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ListResult{Total: total, Offset: filter.Offset, PageSize: filter.PageSize, Sales: sales}, nil
}

func buildFilter(input ListInput) (mysqlInfra.SalesFilter, error) {
	filter := mysqlInfra.SalesFilter{
		ChannelCode:    strings.TrimSpace(input.ChannelCode),
		ConnectionID:   input.ConnectionID,
		OrderStatus:    input.OrderStatus,
		PaymentStatus:  input.PaymentStatus,
		ShippingStatus: input.ShippingStatus,
		Search:         strings.TrimSpace(input.Search),
		Offset:         max(input.Offset, 0),
		PageSize:       input.PageSize,
	}
	if filter.PageSize <= 0 {
		filter.PageSize = defaultPageSize
	}
	filter.PageSize = min(filter.PageSize, maxPageSize)
	if len([]rune(filter.Search)) > maxSearchLength {
		return filter, fmt.Errorf("%w: la búsqueda es demasiado larga", ErrInvalidSalesFilter)
	}

	for _, check := range []struct {
		name, value string
		allowed     []string
	}{
		{"orderStatus", filter.OrderStatus, orderStatuses},
		{"paymentStatus", filter.PaymentStatus, paymentStatuses},
		{"shippingStatus", filter.ShippingStatus, shipStatuses},
	} {
		if check.value != "" && !slices.Contains(check.allowed, check.value) {
			return filter, fmt.Errorf("%w: %s", ErrInvalidSalesFilter, check.name)
		}
	}

	parseDay := func(name, value string) (*time.Time, error) {
		if value == "" {
			return nil, nil
		}
		parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			return nil, fmt.Errorf("%w: %s debe tener el formato AAAA-MM-DD", ErrInvalidSalesFilter, name)
		}
		return &parsed, nil
	}
	var err error
	if filter.From, err = parseDay("from", input.From); err != nil {
		return filter, err
	}
	to, err := parseDay("to", input.To)
	if err != nil {
		return filter, err
	}
	if to != nil {
		until := to.AddDate(0, 0, 1)
		filter.Until = &until
	}
	return filter, nil
}

func (s *QueryService) Detail(ctx context.Context, id int64) (*Detail, error) {
	sale, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	detail := &Detail{SaleDetailDTO: *sale, RelatedSales: []mysqlInfra.RelatedSaleDTO{}}
	if sale.PackID != nil {
		detail.RelatedSales, err = s.repository.FindPackSiblings(ctx, sale.ConnectionID, *sale.PackID, sale.ID)
		if err != nil {
			return nil, err
		}
	}
	detail.EstimatedNetAmount, detail.ShippingCostOnRelatedSale = estimateNet(sale, detail.RelatedSales)
	return detail, nil
}

// estimateNet calcula el neto estimado. El costo de un envío compartido por un
// paquete se resta una sola vez: en la orden guardada primero.
func estimateNet(sale *mysqlInfra.SaleDetailDTO, siblings []mysqlInfra.RelatedSaleDTO) (*float64, bool) {
	sharedElsewhere := len(siblings) > 0 && siblings[0].ID < sale.ID
	if sale.TotalAmount == nil || sale.MarketplaceFeeAmount == nil ||
		sale.OrderStatus == OrderCancelled || sale.PaymentStatus == PaymentRefunded ||
		sale.PaymentStatus == PaymentPartiallyRefunded || sale.PaymentStatus == PaymentRejected {
		return nil, sharedElsewhere
	}
	net := *sale.TotalAmount - *sale.MarketplaceFeeAmount
	if !sharedElsewhere {
		switch {
		case sale.ShippingCostSeller != nil:
			net -= *sale.ShippingCostSeller
		case sale.ShippingStatus != ShippingNotApplicable:
			return nil, false
		}
	}
	return &net, sharedElsewhere
}
