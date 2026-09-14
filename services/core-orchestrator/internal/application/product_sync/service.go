// Package product_sync builds, for a product, the read-only "Sincronización"
// view of the product detail page: one entry per active channel connection
// showing whatever ecom_channel_product_map already has for that product on
// that connection — plus, on connections where allows_multiple_listings is
// true, which of the product's vehicle compatibilities still have no listing
// at all (shown as pending). The one exception is a Nissan-branded product:
// channel_listings.publishReady never fans it out per vehicle fitment (it
// always publishes a single general listing, even on an
// allows_multiple_listings connection), so this view mirrors that — a Nissan
// product is reported as if the connection were single-listing, with no
// per-fitment pending rows, so the UI doesn't promise publications that will
// never be created. Nothing here writes; publishing/republishing stays in
// internal/application/channel_listings.
package product_sync

import (
	"context"
	"errors"
	"fmt"
	"sort"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidInput    = errors.New("invalid product sync input")
	ErrProductNotFound = errors.New("product not found")
)

// maxCompatibilitiesForSync bounds how many vehicle compatibilities are
// loaded per product when computing pending publications, mirroring
// channel_listings.maxCompatibilitiesPerSKU.
const maxCompatibilitiesForSync = 500

// nissanBrandID is ecom_brands.id for "Nissan" in this deployment's seed data
// — hardcoded rather than resolved by name/code, mirroring the identically
// named constants in application/sync and application/channel_listings. A
// Nissan product never fans out into per-fitment listings (see
// channel_listings.publishReady), so this view treats it as single-listing.
const nissanBrandID int64 = 1

type Service struct {
	products                    *mysqlInfra.ProductRepository
	channelConnections          *mysqlInfra.ChannelConnectionRepository
	channelProductMap           *mysqlInfra.ChannelProductMapRepository
	channelCategoryMap          *mysqlInfra.ChannelCategoryMapRepository
	productVehicleCompatibility *mysqlInfra.ProductVehicleCompatibilityRepository
	vehicleFitments             *mysqlInfra.VehicleFitmentsRepository
	brands                      *mysqlInfra.BrandsRepository
}

func NewService(
	products *mysqlInfra.ProductRepository,
	channelConnections *mysqlInfra.ChannelConnectionRepository,
	channelProductMap *mysqlInfra.ChannelProductMapRepository,
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository,
	productVehicleCompatibility *mysqlInfra.ProductVehicleCompatibilityRepository,
	vehicleFitments *mysqlInfra.VehicleFitmentsRepository,
	brands *mysqlInfra.BrandsRepository,
) *Service {
	return &Service{
		products:                    products,
		channelConnections:          channelConnections,
		channelProductMap:           channelProductMap,
		channelCategoryMap:          channelCategoryMap,
		productVehicleCompatibility: productVehicleCompatibility,
		vehicleFitments:             vehicleFitments,
		brands:                      brands,
	}
}

// CompatibilityDTO describes the vehicle a listing (or a still-missing
// listing) is for.
type CompatibilityDTO struct {
	VehicleFitmentID int64  `json:"vehicleFitmentId"`
	BrandName        string `json:"brandName"`
	Model            string `json:"model"`
	YearStart        int    `json:"yearStart"`
	YearEnd          *int   `json:"yearEnd,omitempty"`
	Motor            string `json:"motor,omitempty"`
	Position         string `json:"position,omitempty"`
	Side             string `json:"side,omitempty"`
}

type ListingDTO struct {
	ID                   int64             `json:"id"`
	ListingTitle         *string           `json:"listingTitle,omitempty"`
	ExternalID           *string           `json:"externalId,omitempty"`
	ExternalCategoryID   *string           `json:"externalCategoryId,omitempty"`
	ExternalCategoryName *string           `json:"externalCategoryName,omitempty"`
	Status               string            `json:"status"`
	IsEnabled            bool              `json:"isEnabled"`
	LastSyncedAt         *string           `json:"lastSyncedAt,omitempty"`
	Compatibility        *CompatibilityDTO `json:"compatibility,omitempty"`
}

type ConnectionSyncDTO struct {
	ConnectionID           int64        `json:"connectionId"`
	ChannelName            string       `json:"channelName"`
	ConnectionName         string       `json:"connectionName"`
	Environment            string       `json:"environment"`
	AllowsMultipleListings bool         `json:"allowsMultipleListings"`
	Listings               []ListingDTO `json:"listings"`
	// Pending lists compatibilities that still have no listing at all on this
	// connection — only ever populated when AllowsMultipleListings is true.
	Pending []CompatibilityDTO `json:"pending"`
}

type ProductSyncDTO struct {
	Connections []ConnectionSyncDTO `json:"connections"`
}

func (s *Service) GetSync(ctx context.Context, productID int64) (*ProductSyncDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidInput
	}

	product, err := s.products.FindByID(ctx, productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("error loading product: %w", err)
	}

	// A Nissan product publishes a single general listing regardless of the
	// connection's allows_multiple_listings (see channel_listings.publishReady),
	// so it is never fanned out per vehicle fitment here either.
	isNissan := product.BrandID != nil && *product.BrandID == nissanBrandID

	connections, err := s.channelConnections.FindAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading active channel connections: %w", err)
	}

	fitmentIDs, compatByFitment, err := s.loadCompatibilities(ctx, productID)
	if err != nil {
		return nil, err
	}

	out := &ProductSyncDTO{Connections: make([]ConnectionSyncDTO, 0, len(connections))}

	for _, connection := range connections {
		connectionSync, err := s.buildConnectionSync(ctx, productID, connection, isNissan, fitmentIDs, compatByFitment)
		if err != nil {
			return nil, err
		}
		out.Connections = append(out.Connections, *connectionSync)
	}

	return out, nil
}

// loadCompatibilities returns the product's vehicle compatibilities as an
// ordered, de-duplicated list of fitment ids (first-seen order) plus an
// index by fitment id, used both to label a fitment listing and to compute
// which compatibilities are still pending a listing.
func (s *Service) loadCompatibilities(ctx context.Context, productID int64) ([]int64, map[int64]mysqlInfra.ProductVehicleCompatibilityDTO, error) {
	page, err := s.productVehicleCompatibility.FindByProductID(ctx, productID, 0, maxCompatibilitiesForSync)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading vehicle compatibilities: %w", err)
	}

	fitmentIDs := make([]int64, 0, len(page.Compatibilities))
	compatByFitment := make(map[int64]mysqlInfra.ProductVehicleCompatibilityDTO, len(page.Compatibilities))
	for _, compatibility := range page.Compatibilities {
		if _, seen := compatByFitment[compatibility.VehicleFitmentID]; seen {
			continue
		}
		compatByFitment[compatibility.VehicleFitmentID] = compatibility
		fitmentIDs = append(fitmentIDs, compatibility.VehicleFitmentID)
	}

	return fitmentIDs, compatByFitment, nil
}

func (s *Service) buildConnectionSync(
	ctx context.Context,
	productID int64,
	connection mysqlInfra.ChannelConnectionDTO,
	isNissan bool,
	fitmentIDs []int64,
	compatByFitment map[int64]mysqlInfra.ProductVehicleCompatibilityDTO,
) (*ConnectionSyncDTO, error) {
	// A Nissan product is reported as single-listing even on an
	// allows_multiple_listings connection, so both the flag the UI branches on
	// and the per-fitment pending computation below use this instead of
	// connection.AllowsMultipleListings directly.
	fansOutPerFitment := connection.AllowsMultipleListings && !isNissan

	out := &ConnectionSyncDTO{
		ConnectionID:           connection.ID,
		ChannelName:            connection.ChannelName,
		ConnectionName:         connection.Name,
		Environment:            connection.Environment,
		AllowsMultipleListings: fansOutPerFitment,
		Listings:               make([]ListingDTO, 0),
		Pending:                make([]CompatibilityDTO, 0),
	}

	rows, err := s.channelProductMap.FindAllByProductAndConnection(ctx, productID, connection.ID)
	if err != nil {
		return nil, fmt.Errorf("error loading listings for connection %d: %w", connection.ID, err)
	}

	publishedFitments := make(map[int64]bool, len(rows))
	for _, row := range rows {
		listing, err := s.toListingDTO(ctx, connection.ID, row, compatByFitment)
		if err != nil {
			return nil, err
		}
		out.Listings = append(out.Listings, *listing)
		if row.VehicleFitmentID != nil {
			publishedFitments[*row.VehicleFitmentID] = true
		}
	}

	if fansOutPerFitment {
		for _, fitmentID := range fitmentIDs {
			if publishedFitments[fitmentID] {
				continue
			}
			compatibility, err := s.toCompatibilityDTO(ctx, fitmentID, compatByFitment[fitmentID])
			if err != nil {
				return nil, err
			}
			out.Pending = append(out.Pending, *compatibility)
		}
		sort.SliceStable(out.Pending, func(i, j int) bool {
			a, b := out.Pending[i], out.Pending[j]
			if a.BrandName != b.BrandName {
				return a.BrandName < b.BrandName
			}
			return a.Model < b.Model
		})
	}

	return out, nil
}

func (s *Service) toListingDTO(
	ctx context.Context,
	connectionID int64,
	row mysqlInfra.ChannelProductMapDTO,
	compatByFitment map[int64]mysqlInfra.ProductVehicleCompatibilityDTO,
) (*ListingDTO, error) {
	listing := &ListingDTO{
		ID:                 row.ID,
		ListingTitle:       row.ListingTitle,
		ExternalID:         row.ExternalID,
		ExternalCategoryID: row.ExternalCategoryID,
		Status:             row.Status,
		IsEnabled:          row.IsEnabled,
	}
	if row.LastSyncedAt != nil {
		formatted := row.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
		listing.LastSyncedAt = &formatted
	}

	if row.ExternalCategoryID != nil {
		categoryMap, err := s.channelCategoryMap.FindByExternalCategoryAndConnection(ctx, *row.ExternalCategoryID, connectionID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
			return nil, fmt.Errorf("error resolving external category %q: %w", *row.ExternalCategoryID, err)
		}
		if categoryMap != nil {
			listing.ExternalCategoryName = categoryMap.ExternalCategoryName
		}
	}

	if row.VehicleFitmentID != nil {
		compatibility, err := s.toCompatibilityDTO(ctx, *row.VehicleFitmentID, compatByFitment[*row.VehicleFitmentID])
		if err != nil {
			return nil, err
		}
		listing.Compatibility = compatibility
	}

	return listing, nil
}

// toCompatibilityDTO resolves a fitment id to its display fields. compat is
// the product's own compatibility row for that fitment when known (carries
// motor/position/side); it's the zero value when the fitment came from a
// listing whose compatibility was since removed from the product.
func (s *Service) toCompatibilityDTO(ctx context.Context, fitmentID int64, compat mysqlInfra.ProductVehicleCompatibilityDTO) (*CompatibilityDTO, error) {
	fitment, err := s.vehicleFitments.FindByID(ctx, fitmentID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
			return &CompatibilityDTO{VehicleFitmentID: fitmentID}, nil
		}
		return nil, fmt.Errorf("error loading vehicle fitment %d: %w", fitmentID, err)
	}

	brandName := ""
	brand, err := s.brands.FindByID(ctx, fitment.BrandID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrBrandNotFound) {
		return nil, fmt.Errorf("error loading brand %d: %w", fitment.BrandID, err)
	}
	if brand != nil {
		brandName = brand.Name
	}

	return &CompatibilityDTO{
		VehicleFitmentID: fitmentID,
		BrandName:        brandName,
		Model:            fitment.Model,
		YearStart:        fitment.YearStart,
		YearEnd:          fitment.YearEnd,
		Motor:            compat.Motor,
		Position:         compat.Position,
		Side:             compat.Side,
	}, nil
}
