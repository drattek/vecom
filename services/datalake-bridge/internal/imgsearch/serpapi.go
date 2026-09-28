package imgsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SerpClient busca imágenes en toda la web vía SerpAPI (motor google_images).
// No requiere proyecto de Google Cloud; solo una API key de serpapi.com.
type SerpClient struct {
	http *http.Client
	key  string
}

// NewSerp crea el cliente con la API key de SerpAPI.
func NewSerp(key string) *SerpClient {
	return &SerpClient{http: &http.Client{Timeout: 45 * time.Second}, key: key}
}

type serpResponse struct {
	ImagesResults []struct {
		Thumbnail      string `json:"thumbnail"`
		Original       string `json:"original"`
		OriginalWidth  int    `json:"original_width"`
		OriginalHeight int    `json:"original_height"`
		Title          string `json:"title"`
		Link           string `json:"link"`   // página fuente
		Source         string `json:"source"` // nombre del sitio
	} `json:"images_results"`
	Error string `json:"error"`
}

// Search devuelve hasta num imágenes candidatas para la consulta.
func (c *SerpClient) Search(ctx context.Context, query string, num int) ([]Candidate, error) {
	if num < 1 {
		num = 1
	}
	q := url.Values{}
	q.Set("engine", "google_images")
	q.Set("q", query)
	q.Set("api_key", c.key)
	q.Set("safe", "off")

	reqURL := "https://serpapi.com/search.json?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("buscar %q: %w", query, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out serpResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decodificar respuesta SerpAPI: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("SerpAPI: %s", out.Error)
	}
	var cands []Candidate
	for i, it := range out.ImagesResults {
		if i >= num {
			break
		}
		cands = append(cands, Candidate{
			URL:    it.Original,
			Thumb:  it.Thumbnail,
			Source: it.Link,
			Title:  it.Title,
			Width:  it.OriginalWidth,
			Height: it.OriginalHeight,
		})
	}
	return cands, nil
}
