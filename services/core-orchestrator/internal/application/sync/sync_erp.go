package sync

import (
	"log"

	"core-orchestrator/internal/domain"
)

func (s *SyncService) ProcessERPPageProcessed(event domain.PageProcessedEvent) error {
	log.Printf("ERP page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessERPSyncFailed(event domain.SyncFailedEvent) error {
	log.Printf("ERP sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}
