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
	"net/http"
	"regexp"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	_ "golang.org/x/image/webp"

	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	azureBlob "core-orchestrator/internal/infrastructure/storage/azureblob"
)

const (
	// azureDiskCode is ecom_storage_disks.code of the disk whose credentials
	// (ecom_storage_disk_credentials) point at the Azure Blob container the
	// listing's pictures are uploaded to.
	azureDiskCode = "AZURE_BLOB"

	webpContentType = "image/webp"
	// maxPictureBytes bounds how much of a picture download is read.
	maxPictureBytes = 25 << 20
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

// importImages downloads the listing's pictures, converts each to lossless
// WebP, uploads it to the Azure container and attaches the resulting public URL
// to the product. The product's existing images are kept: the new ones are
// added after them, and one becomes the cover only if the product had none
// (see product_image_import.Service.ImportForProduct). Re-running it reuses
// pictures already uploaded (same blob name -> same URL) instead of downloading
// them again.
//
// Images are supplementary, so any failure here is a warning and never blocks
// linking the listing.
func (s *Service) importImages(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, actorID int64, record func(step, status, detail string)) {
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

	var (
		images   []productImageImportApp.ProductImageItemInput
		uploaded int
		reused   int
		problems []string
	)
	for i, picture := range item.Pictures {
		position := i + 1
		url := client.BlobURL(blobName(product.SKU, product.PartNumber, position))

		if _, err := s.filesRepository.FindByPath(ctx, url); err == nil {
			images = append(images, productImageImportApp.ProductImageItemInput{URL: url})
			reused++
			continue
		} else if !errors.Is(err, mysqlInfra.ErrFileNotFound) {
			problems = append(problems, fmt.Sprintf("imagen %d: %v", position, err))
			continue
		}

		data, err := s.downloadAsWebP(ctx, picture.Source)
		if err != nil {
			problems = append(problems, fmt.Sprintf("imagen %d: %v", position, err))
			continue
		}
		url, err = client.Upload(ctx, blobName(product.SKU, product.PartNumber, position), webpContentType, data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("imagen %d: %v", position, err))
			continue
		}
		images = append(images, productImageImportApp.ProductImageItemInput{URL: url})
		uploaded++
	}

	attached := 0
	if len(images) > 0 {
		result, err := s.productImageImportService.ImportForProduct(ctx, productImageImportApp.ImportForProductInput{
			ProductID:     product.ID,
			StorageDiskID: disk.ID,
			Images:        images,
			ActorID:       actorID,
		})
		if err != nil {
			record(StepImages, StatusWarning, fmt.Sprintf("Las imágenes se subieron a Azure pero no se pudieron vincular al producto: %v", err))
			return
		}
		for _, r := range result.Results {
			if r.Success {
				attached++
			} else {
				problems = append(problems, fmt.Sprintf("%s: %s", r.URL, r.Error))
			}
		}
	}

	detail := fmt.Sprintf("%d de %d imagen(es) agregadas al producto (%d subidas a Azure, %d ya estaban).", attached, len(item.Pictures), uploaded, reused)
	if len(problems) > 0 {
		record(StepImages, StatusWarning, detail+" Con problemas: "+strings.Join(problems, "; "))
		return
	}
	record(StepImages, StatusOK, detail)
}

// downloadAsWebP fetches a picture, decodes it (JPEG, PNG or WebP) and
// re-encodes it as lossless WebP.
func (s *Service) downloadAsWebP(ctx context.Context, pictureURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pictureURL, nil)
	if err != nil {
		return nil, fmt.Errorf("URL inválida: %w", err)
	}

	response, err := s.pictureClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error descargando la imagen: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("la descarga respondió %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxPictureBytes+1))
	if err != nil {
		return nil, fmt.Errorf("error leyendo la imagen: %w", err)
	}
	if len(body) > maxPictureBytes {
		return nil, fmt.Errorf("la imagen supera los %d MB", maxPictureBytes>>20)
	}

	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("formato de imagen no soportado: %w", err)
	}

	var out bytes.Buffer
	if err := nativewebp.Encode(&out, decoded, nil); err != nil {
		return nil, fmt.Errorf("error convirtiendo a WebP: %w", err)
	}
	return out.Bytes(), nil
}
