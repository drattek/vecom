// Package banxico is a read-only client for Banco de México's SIE (Sistema
// de Información Económica) REST API — the only place core-orchestrator
// gets external exchange-rate data from. See
// https://www.banxico.org.mx/SieAPIRest/service/v1/doc/index.html
package banxico

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://www.banxico.org.mx/SieAPIRest/service/v1"

// banxicoDateLayout is the dd/mm/yyyy format SIE reports datapoint dates in.
const banxicoDateLayout = "02/01/2006"

var (
	ErrSIERequestFailed = errors.New("SIE API request failed")
	ErrSIENoData        = errors.New("SIE series has no published value")
)

// Client authenticates every request with a Bmx-Token header — see
// NewClient.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(httpClient *http.Client, token string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{httpClient: httpClient, baseURL: defaultBaseURL, token: token}
}

type seriesResponse struct {
	Bmx struct {
		Series []struct {
			IDSerie string `json:"idSerie"`
			Datos   []struct {
				Fecha string `json:"fecha"`
				Dato  string `json:"dato"`
			} `json:"datos"`
		} `json:"series"`
	} `json:"bmx"`
}

// GetLatestValue fetches the most recently published value for seriesID via
// SIE's "datos/oportuno" endpoint (the series' latest available datapoint,
// no date range needed) and returns it parsed as a float64 alongside the
// date Banxico reports it as of. A non-business day with no value published
// yet ("N/E" in SIE's response) surfaces as ErrSIENoData rather than a
// silent zero rate.
func (c *Client) GetLatestValue(ctx context.Context, seriesID string) (rate float64, asOf time.Time, err error) {
	endpoint := fmt.Sprintf("%s/series/%s/datos/oportuno", c.baseURL, url.PathEscape(seriesID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error building SIE request: %w", err)
	}
	req.Header.Set("Bmx-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error calling SIE API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, time.Time{}, fmt.Errorf("%w: series %s, status %d", ErrSIERequestFailed, seriesID, resp.StatusCode)
	}

	var parsed seriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, time.Time{}, fmt.Errorf("error decoding SIE response for series %s: %w", seriesID, err)
	}

	if len(parsed.Bmx.Series) == 0 || len(parsed.Bmx.Series[0].Datos) == 0 {
		return 0, time.Time{}, fmt.Errorf("%w: series %s", ErrSIENoData, seriesID)
	}

	datum := parsed.Bmx.Series[0].Datos[0]
	dato := strings.TrimSpace(datum.Dato)
	if dato == "" || dato == "N/E" {
		return 0, time.Time{}, fmt.Errorf("%w: series %s has no value for %s", ErrSIENoData, seriesID, datum.Fecha)
	}

	rate, err = strconv.ParseFloat(dato, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error parsing SIE value %q for series %s: %w", dato, seriesID, err)
	}

	asOf, err = time.Parse(banxicoDateLayout, datum.Fecha)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error parsing SIE date %q for series %s: %w", datum.Fecha, seriesID, err)
	}

	return rate, asOf, nil
}
