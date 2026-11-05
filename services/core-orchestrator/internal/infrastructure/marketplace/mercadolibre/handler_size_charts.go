package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// sizeChartResponse is the subset of GET /catalog/charts/{id} this integration
// reads: the chart's rows, each with its own attributes (SIZE, FOOT_LENGTH…).
type sizeChartResponse struct {
	ID   string `json:"id"`
	Rows []struct {
		ID         string `json:"id"`
		Attributes []struct {
			ID     string `json:"id"`
			Values []struct {
				Name string `json:"name"`
			} `json:"values"`
		} `json:"attributes"`
	} `json:"rows"`
}

// GetSizeChartRowSize calls GET /catalog/charts/{chartID} (seller token
// required) and returns the SIZE value name of the chart row rowID (e.g.
// "12364031:4" → "G"). found is false when the chart has no such row or the row
// carries no SIZE attribute.
//
// MercadoLibre's fashion validator requires an item's SIZE to match the SIZE of
// the SIZE_GRID_ROW_ID it points to (invalid.fashion_grid.size.values
// otherwise), so the chart is the source of truth for that value.
func (h *ItemsHandler) GetSizeChartRowSize(ctx context.Context, accessToken, chartID, rowID string) (size string, found bool, err error) {
	requestURL := fmt.Sprintf("%s/catalog/charts/%s", h.client.baseURL, url.PathEscape(chartID))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("error creating mercadolibre size chart request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return "", false, fmt.Errorf("error calling mercadolibre size chart endpoint: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return "", false, fmt.Errorf("error reading mercadolibre size chart response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("mercadolibre size chart %s failed with status %d: %s", chartID, response.StatusCode, payload)
	}

	var chart sizeChartResponse
	if err := json.Unmarshal(payload, &chart); err != nil {
		return "", false, fmt.Errorf("error decoding mercadolibre size chart response: %w", err)
	}

	for _, row := range chart.Rows {
		if row.ID != rowID {
			continue
		}
		for _, attribute := range row.Attributes {
			if attribute.ID == "SIZE" && len(attribute.Values) > 0 {
				if name := strings.TrimSpace(attribute.Values[0].Name); name != "" {
					return name, true, nil
				}
			}
		}
	}

	return "", false, nil
}
