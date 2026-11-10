package product_image_import

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	neturl "net/url"
	"regexp"
	"strings"
	"sync"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ErrInvalidImageURLs lo devuelve ReplaceImagesForProduct cuando alguna URL no
// pasó la validación: no se toca nada y el detalle de cada URL va en el resultado.
var ErrInvalidImageURLs = errors.New("some image urls are invalid; no image was changed")

// inspectConcurrency acota cuántas URLs de una misma fila se validan a la vez.
const inspectConcurrency = 4

type ReplaceImagesInput struct {
	ProductID     int64
	StorageDiskID int64
	// URLs en el orden final de la galería; la primera queda como portada.
	URLs    []string
	ActorID int64
}

// ReplaceImagesForProduct deja las imágenes del producto exactamente como dice
// URLs: da de baja (soft delete) todas las actuales y vincula las nuevas en ese
// orden, la primera como portada. Todas las URLs se validan con un HEAD ANTES de
// tocar nada; si alguna falla, el producto conserva sus imágenes y se devuelve
// ErrInvalidImageURLs junto con el resultado por URL. Los archivos (ecom_files)
// se reutilizan por path, así que repetir la operación no duplica nada. No hay
// columna de posición: el orden de la galería es el id de ecom_product_images,
// que aquí se inserta en el orden pedido.
func (s *Service) ReplaceImagesForProduct(ctx context.Context, input ReplaceImagesInput) (*ImportProductImagesResult, error) {
	if input.ProductID <= 0 || input.StorageDiskID <= 0 || len(input.URLs) == 0 {
		return nil, ErrInvalidImportPayload
	}
	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}
	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}
	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	// Una URL repetida en la fila se vincula una sola vez (la primera posición).
	urls := make([]string, 0, len(input.URLs))
	seen := make(map[string]bool, len(input.URLs))
	for _, raw := range input.URLs {
		url := strings.TrimSpace(raw)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		urls = append(urls, url)
	}
	if len(urls) == 0 {
		return nil, ErrInvalidImportPayload
	}

	// Fase 1: validar todas las URLs (en paralelo acotado) antes de modificar.
	results := make([]ImageImportResult, len(urls))
	pending := make([]pendingImage, len(urls))
	var wg sync.WaitGroup
	slots := make(chan struct{}, inspectConcurrency)
	for i, url := range urls {
		results[i] = ImageImportResult{URL: url, Position: i + 1, IsFirst: i == 0}
		wg.Add(1)
		slots <- struct{}{}
		go func(i int, url string) {
			defer wg.Done()
			defer func() { <-slots }()

			size, contentType, checkErr := s.inspectURL(ctx, url)
			if checkErr != nil {
				results[i].Error = checkErr.Error()
				return
			}
			mimeType, extension, metaErr := resolveImageMeta(url, contentType)
			if metaErr != nil {
				results[i].Error = metaErr.Error()
				return
			}
			pending[i] = pendingImage{
				url:       url,
				isFirst:   i == 0,
				size:      size,
				mimeType:  mimeType,
				extension: extension,
				result:    &results[i],
			}
		}(i, url)
	}
	wg.Wait()

	for i := range results {
		if results[i].Error != "" {
			return &ImportProductImagesResult{Results: results}, ErrInvalidImageURLs
		}
	}

	// Fase 2: una transacción — baja de las actuales + alta de las nuevas.
	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		images := mysqlInfra.NewProductImagesRepository(tx)

		existing, ferr := images.FindAllByProductID(ctx, product.ID)
		if ferr != nil {
			return fmt.Errorf("error listing images of product %d: %w", product.ID, ferr)
		}
		for _, old := range existing {
			if derr := images.SoftDelete(ctx, old.ID); derr != nil && !errors.Is(derr, mysqlInfra.ErrProductImageNotFound) {
				return fmt.Errorf("error removing image %d: %w", old.ID, derr)
			}
		}

		for idx := range pending {
			p := &pending[idx]

			file, ferr := files.FindByPath(ctx, p.url)
			if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
				return fmt.Errorf("error looking up file by path %q: %w", p.url, ferr)
			}
			if file == nil {
				file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
					DiskID:    input.StorageDiskID,
					Path:      p.url,
					Filename:  fmt.Sprintf("%d_%d", product.ID, p.result.Position),
					MimeType:  p.mimeType,
					FileType:  importedFileType,
					Extension: p.extension,
					Size:      p.size,
					IsPublic:  true,
					CreatedBy: actorID,
				})
				if ferr != nil {
					return fmt.Errorf("error creating file record for %q: %w", p.url, ferr)
				}
			}
			p.result.FileID = &file.ID

			link, lerr := images.Create(ctx, mysqlInfra.CreateProductImageInput{
				ProductID: product.ID,
				FileID:    file.ID,
				IsFirst:   p.isFirst,
				CreatedBy: actorID,
			})
			if lerr != nil {
				return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, lerr)
			}
			p.result.ProductImageID = &link.ID
			p.result.Success = true
			p.result.Created = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ImportProductImagesResult{Results: results}, nil
}

// --- YouTube ----------------------------------------------------------------

const (
	// Los videos de YouTube se guardan en ecom_files como una fila más: path =
	// URL canónica del video, file_type 'video', size 0 (no hay archivo) y este
	// mime_type/extension, que es lo que usa el dashboard para saber que debe
	// mostrarlos con el reproductor incrustado de YouTube. Ver ADR 0008.
	YouTubeMimeType  = "video/youtube"
	YouTubeExtension = "youtube"
)

var youTubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

var youTubeHosts = map[string]bool{
	"youtube.com":              true,
	"www.youtube.com":          true,
	"m.youtube.com":            true,
	"music.youtube.com":        true,
	"youtube-nocookie.com":     true,
	"www.youtube-nocookie.com": true,
	"youtu.be":                 true,
	"www.youtu.be":             true,
}

// ParseYouTubeURL reconoce enlaces de YouTube (watch?v=, youtu.be/, shorts/,
// embed/, live/, v/) y los normaliza a https://www.youtube.com/watch?v=ID.
//
//	(canónica, true, nil)  enlace de YouTube válido
//	("", true, err)        es de YouTube pero no trae un ID de video válido
//	("", false, nil)       no es un enlace de YouTube (se trata como archivo)
func ParseYouTubeURL(raw string) (canonical string, isYouTube bool, err error) {
	parsed, perr := neturl.Parse(strings.TrimSpace(raw))
	if perr != nil || !youTubeHosts[strings.ToLower(parsed.Hostname())] {
		return "", false, nil
	}

	id := ""
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	switch {
	case host == "youtu.be":
		id = segments[0]
	case len(segments) >= 2 && (segments[0] == "shorts" || segments[0] == "embed" || segments[0] == "live" || segments[0] == "v"):
		id = segments[1]
	default:
		id = parsed.Query().Get("v")
	}

	if !youTubeID.MatchString(id) {
		return "", true, errors.New("el enlace de YouTube no trae un ID de video válido")
	}
	return "https://www.youtube.com/watch?v=" + id, true, nil
}
