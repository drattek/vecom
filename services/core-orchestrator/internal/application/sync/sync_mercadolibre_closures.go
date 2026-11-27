package sync

import (
	"context"
	"fmt"
	"strings"

	"core-orchestrator/internal/domain"
)

// ListingLiveStatuses consulta en MercadoLibre el estado de las publicaciones de
// la conexión. Es de solo lectura: a diferencia de
// RefreshChannelProductMapStatus, no cambia pausas ni categorías locales.
func (s *MercadoLibreProductSyncService) ListingLiveStatuses(ctx context.Context, connectionID int64) ([]domain.ListingLiveStatus, error) {
	listings, err := s.channelProductMapRepository.FindAllByConnectionID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel product map for connection %d: %w", connectionID, err)
	}

	result := make([]domain.ListingLiveStatus, 0, len(listings))
	externalIDs := make([]string, 0, len(listings))
	seen := make(map[string]bool, len(listings))
	for _, listing := range listings {
		externalID := strings.TrimSpace(derefString(listing.ExternalID))
		if externalID == "" {
			continue
		}
		result = append(result, domain.ListingLiveStatus{
			MapID: listing.ID, ProductID: listing.ProductID, VehicleFitmentID: listing.VehicleFitmentID,
			ExternalID: externalID, LocalStatus: listing.Status,
		})
		if !seen[externalID] {
			seen[externalID] = true
			externalIDs = append(externalIDs, externalID)
		}
	}
	if len(externalIDs) == 0 {
		return result, nil
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}
	statuses, err := s.itemsHandler.GetItemsStatus(ctx, accessToken, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("error fetching mercadolibre item statuses: %w", err)
	}

	live := make(map[string]int, len(statuses))
	for i := range statuses {
		live[statuses[i].ID] = i
	}
	for i := range result {
		index, ok := live[result[i].ExternalID]
		if !ok {
			continue
		}
		status := statuses[index]
		result[i].Found = true
		result[i].RawStatus = status.Status
		result[i].SubStatus = status.SubStatus
		result[i].Closed = foldMercadoLibreStatus(status.Status, status.SubStatus) == mercadoLibreMapStatusClosed
	}
	return result, nil
}

// ClosureReason busca el motivo de la moderación de una publicación. Con
// useHistory consulta además el historial de infracciones: solo debe usarse
// cuando ya se sabe que la cerró MercadoLibre, porque el historial incluye
// infracciones antiguas ya resueltas.
func (s *MercadoLibreProductSyncService) ClosureReason(ctx context.Context, connectionID int64, externalID string, useHistory bool) (domain.ClosureReason, bool, error) {
	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return domain.ClosureReason{}, false, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}

	moderation, err := s.moderationsHandler.LastModeration(ctx, accessToken, externalID)
	if err != nil {
		return domain.ClosureReason{}, false, err
	}
	if moderation == nil && useHistory {
		me, err := s.usersHandler.GetMe(ctx, accessToken)
		if err != nil {
			return domain.ClosureReason{}, false, err
		}
		if moderation, err = s.moderationsHandler.LastInfraction(ctx, accessToken, me.ID, externalID); err != nil {
			return domain.ClosureReason{}, false, err
		}
	}
	if moderation == nil {
		return domain.ClosureReason{}, false, nil
	}
	return domain.ClosureReason{
		ModerationName: moderation.Name, Reason: moderation.Reason, Remedy: moderation.Remedy,
	}, true, nil
}
