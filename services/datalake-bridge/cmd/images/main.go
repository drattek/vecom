// Herramienta aparte (no va en el loop de 30 min) para las fotos de las máquinas.
//
//	fetch:  busca imágenes candidatas por modelo (Google CSE) -> candidates.json
//	upload: sube las imágenes aprobadas a Odoo (image_1920), idempotente
//
// Las fotos son por MODELO (product.template), no por serie. El paso de revisión
// (página aparte) va en medio: el usuario aprueba antes de subir a producción.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"datalake-bridge/internal/imgproc"
	"datalake-bridge/internal/imgsearch"
	"datalake-bridge/internal/odoo"
)

const imageSrcField = "x_image_src"

func main() {
	mode := flag.String("mode", "", "fetch | upload")
	out := flag.String("out", "candidates.json", "fetch: archivo de candidatos a escribir")
	in := flag.String("in", "approved.json", "upload: archivo de aprobados {code:url} o {code:[urls]}")
	candidates := flag.String("candidates", "", "upload: si se da, sube TODAS las candidatas de ese candidates.json")
	num := flag.Int("num", 3, "fetch: candidatos por modelo (máx 10)")
	category := flag.String("category", envOr("ODOO_MACHINE_CATEGORY", "Máquinas"), "categoría de producto")
	flag.Parse()

	oc := odoo.NewClient(mustEnv("ODOO_URL"), mustEnv("ODOO_API_KEY"), mustEnv("ODOO_DATABASE"))
	ctx := context.Background()

	catID, err := oc.EnsureCategory(ctx, *category)
	die(err)

	switch *mode {
	case "fetch":
		die(runFetch(ctx, oc, catID, *out, *num))
	case "upload":
		die(runUpload(ctx, oc, catID, *in, *candidates))
	default:
		fmt.Fprintln(os.Stderr, "usa -mode fetch | -mode upload")
		os.Exit(2)
	}
}

// ---- fetch ----

type modelCandidates struct {
	Code       string                `json:"code"`
	Name       string                `json:"name"`
	Brand      string                `json:"brand"`
	HasImage   bool                  `json:"has_image"`  // ya tiene x_image_src
	Query      string                `json:"query"`      // consulta usada
	Candidates []imgsearch.Candidate `json:"candidates"` // opciones encontradas
}

// searcher abstrae el proveedor de búsqueda de imágenes (SerpAPI o Google CSE).
type searcher interface {
	Search(ctx context.Context, query string, num int) ([]imgsearch.Candidate, error)
}

// newSearcher elige SerpAPI si hay SERPAPI_KEY, si no Google CSE.
func newSearcher() searcher {
	if k := strings.TrimSpace(os.Getenv("SERPAPI_KEY")); k != "" {
		fmt.Println("proveedor de imágenes: SerpAPI")
		return imgsearch.NewSerp(k)
	}
	fmt.Println("proveedor de imágenes: Google CSE")
	return imgsearch.New(mustEnv("GOOGLE_CSE_KEY"), mustEnv("GOOGLE_CSE_CX"))
}

func runFetch(ctx context.Context, oc *odoo.Client, catID int64, outPath string, num int) error {
	search := newSearcher()

	// x_image_src puede no existir aún; solo se lee (dry-run true = no crear).
	srcField, err := oc.EnsureManualCharField(ctx, "product.template", imageSrcField, "Fuente de imagen", true)
	if err != nil {
		return err
	}
	models, err := oc.SearchMachineTemplates(ctx, catID, srcField)
	if err != nil {
		return err
	}
	fmt.Printf("modelos: %d\n", len(models))

	var results []modelCandidates
	for i, m := range models {
		query := buildQuery(m.Brand.Name, m.DefaultCode, m.Name)
		// Reintenta una vez si SerpAPI se tarda; si aun así falla, sigue sin
		// candidatas para ese modelo (el usuario lo resuelve en la revisión).
		cands, err := search.Search(ctx, query, num)
		if err != nil {
			cands, err = search.Search(ctx, query, num)
		}
		if err != nil {
			fmt.Printf("  [%d/%d] %-18s ! error: %v\n", i+1, len(models), m.DefaultCode, err)
		} else {
			fmt.Printf("  [%d/%d] %-18s %q -> %d candidatas\n", i+1, len(models), m.DefaultCode, query, len(cands))
		}
		results = append(results, modelCandidates{
			Code:       m.DefaultCode,
			Name:       m.Name,
			Brand:      m.Brand.Name,
			HasImage:   string(m.ImageSrc) != "",
			Query:      query,
			Candidates: cands,
		})
		time.Sleep(300 * time.Millisecond) // suave con la cuota
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("escrito %s (%d modelos)\n", outPath, len(results))
	return nil
}

// buildQuery arma la consulta: marca + modelo + tipo (primera palabra del nombre).
func buildQuery(brand, code, name string) string {
	parts := []string{}
	if brand != "" {
		parts = append(parts, brand)
	}
	parts = append(parts, code)
	if t := firstWord(name); t != "" {
		parts = append(parts, t)
	}
	return strings.Join(parts, " ")
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i]
	}
	return s
}

// ---- upload ----

func runUpload(ctx context.Context, oc *odoo.Client, catID int64, inPath, candPath string) error {
	desired, err := loadDesired(inPath, candPath)
	if err != nil {
		return err
	}

	srcField, err := oc.EnsureManualCharField(ctx, "product.template", imageSrcField, "Fuente de imagen", false)
	if err != nil {
		return err
	}
	models, err := oc.SearchMachineTemplates(ctx, catID, srcField)
	if err != nil {
		return err
	}
	byCode := map[string]odoo.MachineTemplate{}
	for _, m := range models {
		byCode[m.DefaultCode] = m
	}

	var updated, skipped, failed, imgsUp int
	for code, urls := range desired {
		urls = cleanURLs(urls)
		if len(urls) == 0 {
			continue
		}
		m, ok := byCode[code]
		if !ok {
			fmt.Printf("  ! %s: no existe en Odoo, se omite\n", code)
			failed++
			continue
		}
		// Gate idempotente: la fuente guardada es el JSON de la lista de URLs.
		srcValue, _ := json.Marshal(urls)
		if string(m.ImageSrc) == string(srcValue) {
			skipped++
			continue
		}
		// Baja y procesa todas (blanco + recorte). La portada debe existir.
		var b64s []string
		for _, u := range urls {
			raw, derr := downloadImage(ctx, u)
			if derr != nil {
				fmt.Printf("  ~ %s: se salta una (%v)\n", code, derr)
				continue
			}
			if p, perr := imgproc.WhiteBgTrim(raw); perr == nil {
				raw = p
			}
			b64s = append(b64s, base64.StdEncoding.EncodeToString(raw))
		}
		if len(b64s) == 0 {
			fmt.Printf("  ! %s: ninguna imagen utilizable\n", code)
			failed++
			continue
		}
		// Portada + fuente.
		if err := oc.SetProductImage(ctx, m.ID, b64s[0], string(srcValue), srcField); err != nil {
			fmt.Printf("  ! %s: portada falló (%v)\n", code, err)
			failed++
			continue
		}
		// Galería: reemplaza las existentes por el resto.
		if ids, gerr := oc.ProductImageIDs(ctx, m.ID); gerr == nil {
			_ = oc.DeleteProductImages(ctx, ids)
		}
		for i, b64 := range b64s[1:] {
			if err := oc.CreateProductImage(ctx, m.ID, fmt.Sprintf("%s #%d", code, i+2), b64); err != nil {
				fmt.Printf("  ~ %s: galería #%d falló (%v)\n", code, i+2, err)
			}
		}
		fmt.Printf("  + %s: %d imagen(es)\n", code, len(b64s))
		updated++
		imgsUp += len(b64s)
	}
	fmt.Printf("modelos_actualizados=%d imagenes=%d sin_cambio=%d fallidos=%d\n", updated, imgsUp, skipped, failed)
	return nil
}

// loadDesired arma el mapa modelo -> lista de URLs. Si candPath se da, usa TODAS
// las candidatas de ese candidates.json; si no, lee inPath ({code:url},
// {code:[urls]} o [{code,url}]).
func loadDesired(inPath, candPath string) (map[string][]string, error) {
	if candPath != "" {
		raw, err := os.ReadFile(candPath)
		if err != nil {
			return nil, err
		}
		var cands []struct {
			Code       string `json:"code"`
			Candidates []struct {
				URL string `json:"url"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(raw, &cands); err != nil {
			return nil, fmt.Errorf("candidates.json inválido: %w", err)
		}
		out := map[string][]string{}
		for _, m := range cands {
			for _, c := range m.Candidates {
				out[m.Code] = append(out[m.Code], c.URL)
			}
		}
		return out, nil
	}
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return nil, err
	}
	// {code:[urls]}
	multi := map[string][]string{}
	if err := json.Unmarshal(raw, &multi); err == nil {
		return multi, nil
	}
	// {code:url}
	single := map[string]string{}
	if err := json.Unmarshal(raw, &single); err == nil {
		out := map[string][]string{}
		for k, v := range single {
			out[k] = []string{v}
		}
		return out, nil
	}
	return nil, fmt.Errorf("formato de %s inválido", inPath)
}

func cleanURLs(urls []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

const maxImageBytes = 12 << 20 // 12 MB

func downloadImage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "vegusa-datalake-bridge image-fetch")
	cli := &http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("no es imagen (%s)", ct)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxImageBytes {
		return nil, fmt.Errorf("imagen > 12MB")
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("vacía")
	}
	return body, nil
}

// ---- helpers ----

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
func mustEnv(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		fmt.Fprintf(os.Stderr, "falta la variable %s\n", k)
		os.Exit(1)
	}
	return v
}
func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
