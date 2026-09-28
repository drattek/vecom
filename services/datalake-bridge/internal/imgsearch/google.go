// Package imgsearch busca imágenes candidatas por modelo usando la Custom Search
// JSON API de Google (Programmable Search Engine restringido a dominios de marcas
// y marketplaces de maquinaria). Solo lee; el usuario aprueba antes de subir.
package imgsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Candidate es una imagen candidata para un modelo.
type Candidate struct {
	URL    string `json:"url"`    // imagen a tamaño completo
	Thumb  string `json:"thumb"`  // miniatura (para la página de revisión)
	Source string `json:"source"` // página donde vive la imagen
	Title  string `json:"title"`
	Mime   string `json:"mime"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Client habla con la Custom Search JSON API.
type Client struct {
	http *http.Client
	key  string
	cx   string
}

// New crea el cliente con la API key y el id del buscador (cx).
func New(key, cx string) *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second}, key: key, cx: cx}
}

type apiResponse struct {
	Items []struct {
		Link  string `json:"link"`
		Mime  string `json:"mime"`
		Title string `json:"title"`
		Image struct {
			ThumbnailLink string `json:"thumbnailLink"`
			ContextLink   string `json:"contextLink"`
			Width         int    `json:"width"`
			Height        int    `json:"height"`
		} `json:"image"`
	} `json:"items"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Search devuelve hasta num imágenes candidatas para la consulta.
func (c *Client) Search(ctx context.Context, query string, num int) ([]Candidate, error) {
	if num < 1 {
		num = 1
	}
	if num > 10 {
		num = 10 // tope de la API
	}
	q := url.Values{}
	q.Set("key", c.key)
	q.Set("cx", c.cx)
	q.Set("searchType", "image")
	q.Set("num", strconv.Itoa(num))
	q.Set("q", query)
	q.Set("safe", "off")

	reqURL := "https://www.googleapis.com/customsearch/v1?" + q.Encode()
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

	var out apiResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decodificar respuesta CSE: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("CSE error %d: %s", out.Error.Code, out.Error.Message)
	}
	var cands []Candidate
	for _, it := range out.Items {
		cands = append(cands, Candidate{
			URL:    it.Link,
			Thumb:  it.Image.ThumbnailLink,
			Source: it.Image.ContextLink,
			Title:  it.Title,
			Mime:   it.Mime,
			Width:  it.Image.Width,
			Height: it.Image.Height,
		})
	}
	return cands, nil
}
