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
// resumen de imágenes, stock, precio, conexiones de Odoo/MercadoLibre y si les
// faltan atributos requeridos de la categoría de MercadoLibre.
func (s *Service) PublicationsReport(ctx context.Context, f mysqlInfra.PublicationsReportFilter) (*PublicationsReportResult, error) {
	page, err := s.publications.FindPublicationsReport(ctx, f)
	if err != nil {
		return nil, err
	}
	summary, err := s.publications.FindPublicationsSummary(ctx, f)
	if err != nil {
		return nil, err
	}

	s.attachMeliAttributes(ctx, page.Rows)

	return &PublicationsReportResult{PaginatedPublicationsReport: *page, Summary: summary}, nil
}

// attachMeliAttributes calcula, por cada conexión viva de MercadoLibre de cada
// fila, si faltan atributos requeridos. Un fallo puntual se marca como "error"
// en esa conexión en vez de tumbar todo el reporte.
func (s *Service) attachMeliAttributes(ctx context.Context, rows []mysqlInfra.PublicationReportRow) {
	// Solo conexiones vivas: una publicación cerrada ya no se completa.
	type job struct{ row, slot int }
	jobs := make([]job, 0)
	live := make([][]mysqlInfra.PublicationReportConnection, len(rows))
	for i := range rows {
		for _, c := range rows[i].MercadoLibre {
			if c.State != mysqlInfra.ListingStateClosed {
				live[i] = append(live[i], c)
			}
		}
		rows[i].MeliAttributes = make([]mysqlInfra.PublicationReportAttributeStatus, len(live[i]))
		for j := range live[i] {
			jobs = append(jobs, job{i, j})
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, checklistConcurrency)
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("publications report: checklist panic: %v", rec)
					rows[j.row].MeliAttributes[j.slot] = attrError(live[j.row][j.slot])
				}
			}()

			rows[j.row].MeliAttributes[j.slot] = s.statusFor(ctx, rows[j.row].ProductID, live[j.row][j.slot])
		}(j)
	}
	wg.Wait()
}

func (s *Service) statusFor(ctx context.Context, productID int64, conn mysqlInfra.PublicationReportConnection) mysqlInfra.PublicationReportAttributeStatus {
	checklist, err := s.checklist.GetChecklist(ctx, productID, conn.ConnectionID)
	if err != nil {
		log.Printf("publications report: checklist product=%d connection=%d: %v", productID, conn.ConnectionID, err)
		return attrError(conn)
	}

	out := mysqlInfra.PublicationReportAttributeStatus{
		ConnectionID:   conn.ConnectionID,
		ConnectionName: conn.ConnectionName,
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

func attrError(conn mysqlInfra.PublicationReportConnection) mysqlInfra.PublicationReportAttributeStatus {
	return mysqlInfra.PublicationReportAttributeStatus{
		ConnectionID:   conn.ConnectionID,
		ConnectionName: conn.ConnectionName,
		State:          attrStateError,
		MissingNames:   []string{},
	}
}
