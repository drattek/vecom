package sync

import (
	"context"
	"log"
	"strings"
)

// mercadoLibreSubStatusOutOfStock marca la pausa que MercadoLibre aplica solo
// por falta de stock; la quita él mismo al recibir stock. Es distinta de
// paused_by_seller, que no se reactiva sola.
const mercadoLibreSubStatusOutOfStock = "out_of_stock"

const pausedReasonUser = "user"

func hasSubStatus(subStatus []string, wanted string) bool {
	for _, value := range subStatus {
		if strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}
	return false
}

// pausedReasonFor decide el paused_reason al reflejar el estado que reporta
// MercadoLibre. status usa el vocabulario de ecom_channel_product_map.
//
//   - Una pausa propia por falta de stock se conserva mientras siga pausada o
//     pase por revisión: sin ese motivo nadie la reactiva.
//   - La pausa out_of_stock de MercadoLibre queda sin motivo.
//   - Cualquier otra pausa es del vendedor ('user') y no se reactiva.
func pausedReasonFor(status, previousReason string, subStatus []string) string {
	switch status {
	case mercadoLibreMapStatusPaused:
		switch {
		case previousReason == pausedReasonZeroStock:
			return pausedReasonZeroStock
		case hasSubStatus(subStatus, mercadoLibreSubStatusOutOfStock):
			return ""
		default:
			return pausedReasonUser
		}
	case mercadoLibreMapStatusUnderReview:
		if previousReason == pausedReasonZeroStock {
			return pausedReasonZeroStock
		}
	}
	return ""
}

// liveListingState lee el estado actual de una publicación. ok es false si no se
// pudo leer; entonces status es fallback.
func (s *MercadoLibreProductSyncService) liveListingState(ctx context.Context, accessToken, externalID, fallback string) (status string, subStatus []string, ok bool) {
	live, err := s.itemsHandler.GetItem(ctx, accessToken, externalID)
	if err != nil {
		log.Printf("mercadolibre: listing %s — could not re-read its status after a push, keeping %q: %v", externalID, fallback, err)
		return fallback, nil, false
	}
	return foldMercadoLibreStatus(live.Status, live.SubStatus), live.SubStatus, true
}

// stateAfterZeroStockPush se usa al enviar stock 0 a una publicación activa sin
// pausarla por cuenta propia: MercadoLibre puede haberla pausado solo. Devuelve
// el estado y el motivo a guardar.
func (s *MercadoLibreProductSyncService) stateAfterZeroStockPush(ctx context.Context, accessToken, externalID string) (status, pausedReason string) {
	live, subStatus, ok := s.liveListingState(ctx, accessToken, externalID, mercadoLibreMapStatusSynced)
	if !ok || live != mercadoLibreMapStatusPaused {
		return mercadoLibreMapStatusSynced, ""
	}
	return live, pausedReasonFor(live, "", subStatus)
}
