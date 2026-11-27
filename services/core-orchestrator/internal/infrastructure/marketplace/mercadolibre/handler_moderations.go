package mercadolibre

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxModerationResponseBytes = 1 << 20

// ModerationsHandler consulta las moderaciones de MercadoLibre.
type ModerationsHandler struct {
	client *Client
}

func NewModerationsHandler(client *Client) *ModerationsHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}
	return &ModerationsHandler{client: client}
}

// Moderation es el motivo de una moderación. Remedy no viene en las bajas
// definitivas.
type Moderation struct {
	Name   string
	Reason string
	Remedy string
}

type lastModerationEntry struct {
	Name     string `json:"name"`
	Wordings []struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"wordings"`
}

// LastModeration llama GET /moderations/last_moderation/{item}-ITM: la
// moderación vigente de la publicación. Devuelve (nil, nil) si no tiene.
func (h *ModerationsHandler) LastModeration(ctx context.Context, accessToken, itemID string) (*Moderation, error) {
	body, found, err := h.get(ctx, accessToken, "/moderations/last_moderation/"+url.PathEscape(itemID)+"-ITM")
	if err != nil || !found {
		return nil, err
	}

	// La documentación muestra una lista; se admite también un objeto suelto.
	var entries []lastModerationEntry
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '{' {
		var single lastModerationEntry
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return nil, fmt.Errorf("error decoding mercadolibre last moderation response: %w", err)
		}
		entries = []lastModerationEntry{single}
	} else if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre last moderation response: %w", err)
	}

	for _, entry := range entries {
		moderation := Moderation{Name: entry.Name}
		for _, wording := range entry.Wordings {
			switch strings.ToUpper(wording.Type) {
			case "REASON":
				moderation.Reason = strings.TrimSpace(wording.Value)
			case "REMEDY":
				moderation.Remedy = strings.TrimSpace(wording.Value)
			}
		}
		if moderation.Reason != "" || moderation.Name != "" {
			return &moderation, nil
		}
	}
	return nil, nil
}

// LastInfraction llama GET /moderations/infractions/{user}: el historial de
// infracciones, aquí la más reciente de la publicación, con textos en español.
// Devuelve (nil, nil) si no hay.
func (h *ModerationsHandler) LastInfraction(ctx context.Context, accessToken string, userID int64, itemID string) (*Moderation, error) {
	query := url.Values{}
	query.Set("related_item_id", itemID)
	query.Set("element_type", "ITM")
	query.Set("language", "ES")
	query.Set("sort", "date_created_desc")
	query.Set("limit", "1")

	body, found, err := h.get(ctx, accessToken, "/moderations/infractions/"+strconv.FormatInt(userID, 10)+"?"+query.Encode())
	if err != nil || !found {
		return nil, err
	}

	var payload struct {
		Infractions []struct {
			RelatedItemID string `json:"related_item_id"`
			FilterGroup   string `json:"filter_subgroup"`
			Reason        string `json:"reason"`
			Remedy        string `json:"remedy"`
		} `json:"infractions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre infractions response: %w", err)
	}
	for _, infraction := range payload.Infractions {
		if infraction.RelatedItemID == itemID && infraction.Reason != "" {
			return &Moderation{
				Name:   infraction.FilterGroup,
				Reason: strings.TrimSpace(infraction.Reason),
				Remedy: strings.TrimSpace(infraction.Remedy),
			}, nil
		}
	}
	return nil, nil
}

// get devuelve found=false ante un 404.
func (h *ModerationsHandler) get(ctx context.Context, accessToken, path string) (body []byte, found bool, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, h.client.baseURL+path, nil)
	if err != nil {
		return nil, false, fmt.Errorf("error creating mercadolibre moderations request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, false, fmt.Errorf("error calling mercadolibre moderations endpoint: %w", err)
	}
	defer response.Body.Close()

	body, err = io.ReadAll(io.LimitReader(response.Body, maxModerationResponseBytes))
	if err != nil {
		return nil, false, fmt.Errorf("error reading mercadolibre moderations response: %w", err)
	}
	switch response.StatusCode {
	case http.StatusOK:
		return body, true, nil
	case http.StatusNotFound:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("mercadolibre moderations failed: %w", parseItemAPIError(response.StatusCode, body))
	}
}
