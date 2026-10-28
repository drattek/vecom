package reports

import (
	"context"
	"log"
	"sync"

	checklistApp "core-orchestrator/internal/application/product_attributes_checklist"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// checklistConcurrency acota cuántos checklists de atributos se calculan en
// paralelo por página; cada uno hace varias consultas.
const checklistConcurrency = 8

const (
	attrStateOK            = "ok"
	attrStateNeedsCategory = "needs_category"
	attrStateError         = "error"
)

type Service struct {
	publications *mysqlInfra.PublicationsReportRepository
	checklist    *checklistApp.Service
}

func NewService(publications *mysqlInfra.PublicationsReportRepository, checklist *checklistApp.Service) *Service {
	return &Service{publications: publications, checklist: checklist}
}

// PublicationsReportResult es la página de la tabla más los totales de las
// cards, ambos calculados con los mismos filtros.
type PublicationsReportResult struct {
	mysqlInfra.PaginatedPublicationsReport
	Summary *mysqlInfra.PublicationsReportSummary `json:"summary"`
}

// PublicationsReport lista los productos publicados en algún canal con su
// resumen de imágenes, stock, precio y conexiones de Odoo/MercadoLibre. Los
// atributos requeridos de MercadoLibre se piden aparte (AttributeStatuses): son
// caros y no deben retrasar la tabla ni las cards.
func (s *Service) PublicationsReport(ctx context.Context, f mysqlInfra.PublicationsReportFilter) (*PublicationsReportResult, error) {
	page, err := s.publications.FindPublicationsReport(ctx, f)
	if err != nil {
		return nil, err
	}
	summary, err := s.publications.FindPublicationsSummary(ctx, f)
	if err != nil {
		return nil, err
	}

	return &PublicationsReportResult{PaginatedPublicationsReport: *page, Summary: summary}, nil
}

// AttributePair identifica un producto y la conexión de MercadoLibre cuyo
// checklist de atributos se quiere evaluar.
type AttributePair struct {
	ProductID    int64
	ConnectionID int64
}

// AttributeStatusResult es el estado de atributos de un producto en una conexión.
type AttributeStatusResult struct {
	ProductID int64 `json:"productId"`
	mysqlInfra.PublicationReportAttributeStatus
}

// AttributeStatuses calcula, por cada par producto/conexión, si faltan
// atributos requeridos. Un fallo puntual se marca como "error" en ese par en
// vez de tumbar todo el lote.
func (s *Service) AttributeStatuses(ctx context.Context, pairs []AttributePair) []AttributeStatusResult {
	results := make([]AttributeStatusResult, len(pairs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, checklistConcurrency)
	for i, pair := range pairs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, pair AttributePair) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("publications report: checklist panic: %v", rec)
					results[i] = AttributeStatusResult{ProductID: pair.ProductID, PublicationReportAttributeStatus: attrError(pair.ConnectionID, "")}
				}
			}()

			results[i] = AttributeStatusResult{ProductID: pair.ProductID, PublicationReportAttributeStatus: s.statusFor(ctx, pair)}
		}(i, pair)
	}
	wg.Wait()
	return results
}

func (s *Service) statusFor(ctx context.Context, pair AttributePair) mysqlInfra.PublicationReportAttributeStatus {
	checklist, err := s.checklist.GetChecklist(ctx, pair.ProductID, pair.ConnectionID)
	if err != nil {
		log.Printf("publications report: checklist product=%d connection=%d: %v", pair.ProductID, pair.ConnectionID, err)
		return attrError(pair.ConnectionID, "")
	}

	out := mysqlInfra.PublicationReportAttributeStatus{
		ConnectionID:   pair.ConnectionID,
		ConnectionName: checklist.ConnectionName,
		State:          attrStateOK,
		MissingNames:   []string{},
	}
	if checklist.State == checklistApp.StateNeedsSelection {
		out.State = attrStateNeedsCategory
		return out
	}
	for _, item := range checklist.Items {
		if item.IsRequired && item.Value == "" && item.OptionID == nil {
			name := item.Name
			if item.ExternalLabel != nil && *item.ExternalLabel != "" {
				name = *item.ExternalLabel
			}
			out.MissingNames = append(out.MissingNames, name)
		}
	}
	out.RequiredMissing = len(out.MissingNames)
	return out
}

func attrError(connectionID int64, connectionName string) mysqlInfra.PublicationReportAttributeStatus {
	return mysqlInfra.PublicationReportAttributeStatus{
		ConnectionID:   connectionID,
		ConnectionName: connectionName,
		State:          attrStateError,
		MissingNames:   []string{},
	}
}
