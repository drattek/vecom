package meli_listing_import

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/HugoSmits86/nativewebp"
	_ "golang.org/x/image/webp"

	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	azureBlob "core-orchestrator/internal/infrastructure/storage/azureblob"
	"core-orchestrator/internal/shared/safe"
)

const (
	// azureDiskCode is ecom_storage_disks.code of the disk whose credentials
	// (ecom_storage_disk_credentials) point at the Azure Blob container the
	// listing's pictures are uploaded to.
	azureDiskCode = "AZURE_BLOB"

	webpContentType = "image/webp"
	// maxPictureBytes bounds how much of a picture download is read.
	maxPictureBytes = 25 << 20

	// imageWorkers is how many pictures are downloaded, converted and uploaded
	// at once. Lossless WebP encoding is CPU-heavy and sequentially a listing
	// with a dozen pictures takes well over a minute, longer than a typical
	// proxy waits for a response.
	imageWorkers = 4
	// uploadTimeout bounds a single blob upload.
	uploadTimeout = 60 * time.Second
)

var blobNameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// blobName is "{sku}_{part_number}_MELI_{n}.webp", with anything outside
// letters, digits, dot, underscore and hyphen in sku / part number replaced by a
// hyphen so the name is safe in a URL. n is the picture's 1-based position in the
// listing, so a failed picture doesn't shift the names of the next ones.
func blobName(sku, partNumber string, n int) string {
	clean := func(s string) string { return strings.Trim(blobNameUnsafe.ReplaceAllString(strings.TrimSpace(s), "-"), "-") }
	return fmt.Sprintf("%s_%s_MELI_%d.webp", clean(sku), clean(partNumber), n)
}

// pictureOutcome is what processing one picture produced, kept per position so
// the product's images are attached in the listing's order.
type pictureOutcome struct {
	url      string
	uploaded bool
	reused   bool
	err      error
}

// importImages downloads the listing's pictures, converts each to lossless
// WebP, uploads it to the Azure container and attaches the resulting public URL
// to the product. The product's existing images are kept: the new ones are
// added after them, and one becomes the cover only if the product had none
// (see product_image_import.Service.ImportForProduct). Re-running it reuses
// pictures already uploaded (same blob name -> same URL) instead of downloading
// them again. Up to imageWorkers pictures are processed at once, and every
// phase of every picture is logged with its duration, so a slow or failing
// import shows exactly where it stalled.
//
// Images are supplementary, so any failure here is a warning and never blocks
// linking the listing.
func (s *Service) importImages(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, actorID int64, record func(step, status, detail string)) {
	logf := func(format string, args ...any) {
		log.Printf("meli listing import: product=%d item=%s images: "+format, append([]any{product.ID, item.ID}, args...)...)
	}

	if len(item.Pictures) == 0 {
		record(StepImages, StatusSkipped, "La publicación no tiene imágenes.")
		return
	}

	disk, err := s.storageDiskRepository.FindByCode(ctx, azureDiskCode)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			record(StepImages, StatusWarning, fmt.Sprintf("No existe el disco de almacenamiento %q; no se importaron imágenes.", azureDiskCode))
			return
		}
		record(StepImages, StatusWarning, fmt.Sprintf("No se pudo cargar el disco de almacenamiento: %v", err))
		return
	}

	credentials, err := s.storageDiskCredentialsRepository.FindByDiskID(ctx, disk.ID)
	if err != nil {
		record(StepImages, StatusWarning, fmt.Sprintf("No se pudieron leer las credenciales de Azure: %v", err))
		return
	}
	client, err := azureBlob.NewClient(credentials)
	if err != nil {
		record(StepImages, StatusWarning, fmt.Sprintf("Credenciales de Azure no disponibles: %v", err))
		return
	}

	total := len(item.Pictures)
	logf("processing %d picture(s) with %d worker(s)", total, imageWorkers)

	outcomes := make([]pictureOutcome, total)
	slots := make(chan struct{}, imageWorkers)
	var wg sync.WaitGroup
	for i := range item.Pictures {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-slots }()
			if !safe.Do("meli-listing-import-picture", func() {
				outcomes[i] = s.processPicture(ctx, client, product, item.Pictures[i], i+1, total, logf)
			}) {
				outcomes[i] = pictureOutcome{err: errors.New("error interno al procesar la imagen (ver el log del servidor)")}
			}
		}(i)
	}
	wg.Wait()

	var (
		images   []productImageImportApp.ProductImageItemInput
		uploaded int
		reused   int
		problems []string
	)
	for i, outcome := range outcomes {
		switch {
		case outcome.err != nil:
			problems = append(problems, fmt.Sprintf("imagen %d: %v", i+1, outcome.err))
		default:
			images = append(images, productImageImportApp.ProductImageItemInput{URL: outcome.url})
			if outcome.uploaded {
				uploaded++
			} else if outcome.reused {
				reused++
			}
		}
	}

	attached := 0
	if len(images) > 0 {
		started := time.Now()
		result, err := s.productImageImportService.ImportForProduct(ctx, productImageImportApp.ImportForProductInput{
			ProductID:     product.ID,
			StorageDiskID: disk.ID,
			Images:        images,
			ActorID:       actorID,
		})
		if err != nil {
			logf("linking %d image(s) to the product failed after %s: %v", len(images), time.Since(started).Round(time.Millisecond), err)
			record(StepImages, StatusWarning, fmt.Sprintf("Las imágenes se subieron a Azure pero no se pudieron vincular al producto: %v", err))
			return
		}
		logf("linked %d image(s) to the product in %s", len(images), time.Since(started).Round(time.Millisecond))
		for _, r := range result.Results {
			if r.Success {
				attached++
			} else {
				problems = append(problems, fmt.Sprintf("%s: %s", r.URL, r.Error))
			}
		}
	}

	detail := fmt.Sprintf("%d de %d imagen(es) agregadas al producto (%d subidas a Azure, %d ya estaban).", attached, total, uploaded, reused)
	if len(problems) > 0 {
		record(StepImages, StatusWarning, detail+" Con problemas: "+strings.Join(problems, "; "))
		return
	}
	record(StepImages, StatusOK, detail)
}

// processPicture takes one listing picture all the way to a public Azure URL:
// skip it if that blob was already imported, otherwise download, convert to
// WebP and upload. Each phase is logged with its duration and, on failure, which
// phase failed — the error returned names it too, so it reaches the dashboard.
func (s *Service) processPicture(ctx context.Context, client *azureBlob.Client, product *mysqlInfra.ProductDTO, picture mercadoLibreInfra.ItemPicture, position, total int, logf func(format string, args ...any)) pictureOutcome {
	name := blobName(product.SKU, product.PartNumber, position)
	url := client.BlobURL(name)
	tag := fmt.Sprintf("[%d/%d %s]", position, total, name)

	if _, err := s.filesRepository.FindByPath(ctx, url); err == nil {
		logf("%s already imported, reusing", tag)
		return pictureOutcome{url: url, reused: true}
	} else if !errors.Is(err, mysqlInfra.ErrFileNotFound) {
		logf("%s lookup failed: %v", tag, err)
		return pictureOutcome{err: fmt.Errorf("al buscar si ya estaba importada: %w", err)}
	}

	source := picture.DownloadURL()
	started := time.Now()
	data, err := s.downloadAsWebP(ctx, source, logf, tag)
	if err != nil {
		logf("%s download/convert failed after %s: %v", tag, time.Since(started).Round(time.Millisecond), err)
		return pictureOutcome{err: err}
	}
	logf("%s downloaded and converted to WebP in %s (%d KB)", tag, time.Since(started).Round(time.Millisecond), len(data)>>10)

	started = time.Now()
	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	url, err = client.Upload(uploadCtx, name, webpContentType, data)
	if err != nil {
		logf("%s upload to Azure failed after %s: %v", tag, time.Since(started).Round(time.Millisecond), err)
		return pictureOutcome{err: fmt.Errorf("al subirla a Azure: %w", err)}
	}
	logf("%s uploaded to Azure in %s", tag, time.Since(started).Round(time.Millisecond))

	return pictureOutcome{url: url, uploaded: true}
}

// downloadAsWebP fetches a picture, decodes it (JPEG, PNG or WebP) and
// re-encodes it as lossless WebP. Errors say which of the three phases failed.
func (s *Service) downloadAsWebP(ctx context.Context, pictureURL string, logf func(format string, args ...any), tag string) ([]byte, error) {
	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pictureURL, nil)
	if err != nil {
		return nil, fmt.Errorf("URL de la imagen inválida (%q): %w", pictureURL, err)
	}

	response, err := s.pictureClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("al descargarla: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("al descargarla: respondió %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxPictureBytes+1))
	if err != nil {
		return nil, fmt.Errorf("al descargarla: %w", err)
	}
	if len(body) > maxPictureBytes {
		return nil, fmt.Errorf("al descargarla: supera los %d MB", maxPictureBytes>>20)
	}
	logf("%s downloaded %d KB in %s", tag, len(body)>>10, time.Since(started).Round(time.Millisecond))

	started = time.Now()
	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("al leer el formato de la imagen: %w", err)
	}
	logf("%s decoded %dx%d in %s", tag, decoded.Bounds().Dx(), decoded.Bounds().Dy(), time.Since(started).Round(time.Millisecond))

	started = time.Now()
	var out bytes.Buffer
	if err := nativewebp.Encode(&out, decoded, nil); err != nil {
		return nil, fmt.Errorf("al convertirla a WebP: %w", err)
	}
	logf("%s encoded to WebP in %s", tag, time.Since(started).Round(time.Millisecond))
	return out.Bytes(), nil
}
