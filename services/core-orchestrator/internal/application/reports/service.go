package reports

import (
	"context"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type Service struct {
	publications *mysqlInfra.PublicationsReportRepository
}

func NewService(publications *mysqlInfra.PublicationsReportRepository) *Service {
	return &Service{publications: publications}
}

// PublicationsReport lista los productos publicados en algún canal con su
// resumen de imágenes, stock, precio y conexiones de Odoo/MercadoLibre.
func (s *Service) PublicationsReport(ctx context.Context, f mysqlInfra.PublicationsReportFilter) (*mysqlInfra.PaginatedPublicationsReport, error) {
	return s.publications.FindPublicationsReport(ctx, f)
}
