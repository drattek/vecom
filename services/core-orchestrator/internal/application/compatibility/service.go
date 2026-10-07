package compatibility

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidEquipmentType          = errors.New("invalid equipment type payload")
	ErrInvalidVehicleFitment         = errors.New("invalid vehicle fitment payload")
	ErrInvalidEquipmentFitment       = errors.New("invalid equipment fitment payload")
	ErrEquipmentFitmentAlreadyExists = errors.New("equipment fitment already exists")
	ErrInvalidProductCompatibility   = errors.New("invalid product compatibility payload")
)

const (
	minFitmentYear = 1900
	maxFitmentYear = 2100
)

// InUseError is returned when deleting a catalog row (fitment or equipment
// type) that other rows still depend on. Count is how many; fitment deletes can
// be retried with force to unlink the products first.
type InUseError struct {
	Kind  string
	Count int64
}

func (e *InUseError) Error() string {
	return fmt.Sprintf("%s is in use by %d record(s)", e.Kind, e.Count)
}

// normalizeOptional trims s and maps blank to nil so "" and NULL are the same
// "absent" value when comparing fitments by natural key.
func normalizeOptional(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// resolveBrandID returns the id of the brand identified by id or, when id is
// zero, by name — creating the brand if no live brand has that name.
func resolveBrandID(ctx context.Context, repo *mysqlInfra.BrandsRepository, brandID int64, brandName string, actorID int64) (int64, error) {
	if brandID > 0 {
		if _, err := repo.FindByID(ctx, brandID); err != nil {
			if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
				return 0, mysqlInfra.ErrVehicleFitmentInvalidReference
			}
			return 0, err
		}
		return brandID, nil
	}

	name := strings.TrimSpace(brandName)
	if name == "" {
		return 0, ErrInvalidVehicleFitment
	}
	brand, err := repo.FindByName(ctx, name)
	if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
		brand, err = repo.Create(ctx, mysqlInfra.CreateBrandInput{Name: name, CreatedBy: actorID})
	}
	if err != nil {
		return 0, err
	}
	return brand.ID, nil
}

type EquipmentTypeService struct {
	db                 *sql.DB
	repository         *mysqlInfra.EquipmentTypesRepository
	fitmentsRepository *mysqlInfra.EquipmentFitmentsRepository
}

func NewEquipmentTypeService(db *sql.DB, repository *mysqlInfra.EquipmentTypesRepository, fitmentsRepository *mysqlInfra.EquipmentFitmentsRepository) *EquipmentTypeService {
	return &EquipmentTypeService{db: db, repository: repository, fitmentsRepository: fitmentsRepository}
}

func (s *EquipmentTypeService) GetPaginatedEquipmentTypes(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedEquipmentTypes, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *EquipmentTypeService) GetEquipmentTypeByID(ctx context.Context, id int64) (*mysqlInfra.EquipmentTypeDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidEquipmentType
	}
	return s.repository.FindByID(ctx, id)
}

func (s *EquipmentTypeService) CreateEquipmentType(ctx context.Context, input mysqlInfra.CreateEquipmentTypeInput) (*mysqlInfra.EquipmentTypeDTO, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.CreatedBy <= 0 {
		return nil, ErrInvalidEquipmentType
	}
	if _, err := s.repository.FindByName(ctx, input.Name); err == nil {
		return nil, mysqlInfra.ErrEquipmentTypeAlreadyExists
	} else if !errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
		return nil, err
	}
	return s.repository.Create(ctx, input)
}

func (s *EquipmentTypeService) UpdateEquipmentType(ctx context.Context, id int64, input mysqlInfra.UpdateEquipmentTypeInput) (*mysqlInfra.EquipmentTypeDTO, error) {
	input.Name = strings.TrimSpace(input.Name)
	if id <= 0 || input.Name == "" || input.UpdatedBy <= 0 {
		return nil, ErrInvalidEquipmentType
	}
	if existing, err := s.repository.FindByName(ctx, input.Name); err == nil && existing.ID != id {
		return nil, mysqlInfra.ErrEquipmentTypeAlreadyExists
	} else if err != nil && !errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
		return nil, err
	}
	return s.repository.Update(ctx, id, input)
}

// DeleteEquipmentType refuses to delete a type that live equipment fitments
// still use: they'd lose their type (and drop out of the Odoo/ML exports,
// which join on it).
func (s *EquipmentTypeService) DeleteEquipmentType(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidEquipmentType
	}
	count, err := s.fitmentsRepository.CountByEquipmentType(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return &InUseError{Kind: "equipment type", Count: count}
	}
	return s.repository.SoftDelete(ctx, id)
}

type VehicleFitmentService struct {
	db                                    *sql.DB
	repository                            *mysqlInfra.VehicleFitmentsRepository
	brandRepository                       *mysqlInfra.BrandsRepository
	productRepository                     *mysqlInfra.ProductRepository
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository
	pendingRepository                     *mysqlInfra.PendingProductVehicleFitmentsRepository
}

func NewVehicleFitmentService(
	db *sql.DB,
	repository *mysqlInfra.VehicleFitmentsRepository,
	brandRepository *mysqlInfra.BrandsRepository,
	productRepository *mysqlInfra.ProductRepository,
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository,
	pendingRepository *mysqlInfra.PendingProductVehicleFitmentsRepository,
) *VehicleFitmentService {
	return &VehicleFitmentService{
		db:                                    db,
		repository:                            repository,
		brandRepository:                       brandRepository,
		productRepository:                     productRepository,
		productVehicleCompatibilityRepository: productVehicleCompatibilityRepository,
		pendingRepository:                     pendingRepository,
	}
}

func (s *VehicleFitmentService) GetPaginatedFitments(ctx context.Context, filter mysqlInfra.VehicleFitmentFilter, offset, pageSize int) (*mysqlInfra.PaginatedVehicleFitments, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.Search(ctx, filter, offset, pageSize)
}

func (s *VehicleFitmentService) GetFitmentByID(ctx context.Context, id int64) (*mysqlInfra.VehicleFitmentDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidVehicleFitment
	}
	return s.repository.FindByID(ctx, id)
}

// CreateFitment creates a fitment, failing with ErrVehicleFitmentAlreadyExists
// when one with the same brand/model/years exists (use FindOrCreateFitment to
// reuse it instead).
func (s *VehicleFitmentService) CreateFitment(ctx context.Context, input mysqlInfra.CreateVehicleFitmentInput) (*mysqlInfra.VehicleFitmentDTO, error) {
	input.Model = strings.TrimSpace(input.Model)
	if err := validateVehicleFitment(input.BrandID, input.Model, input.YearStart, input.YearEnd, input.CreatedBy); err != nil {
		return nil, err
	}
	if _, err := s.repository.FindByUniqueKey(ctx, input.BrandID, input.Model, input.YearStart, input.YearEnd); err == nil {
		return nil, mysqlInfra.ErrVehicleFitmentAlreadyExists
	} else if !errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
		return nil, err
	}
	return s.repository.Create(ctx, input)
}

func (s *VehicleFitmentService) UpdateFitment(ctx context.Context, id int64, input mysqlInfra.UpdateVehicleFitmentInput) (*mysqlInfra.VehicleFitmentDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidVehicleFitment
	}
	input.Model = strings.TrimSpace(input.Model)
	if err := validateVehicleFitment(input.BrandID, input.Model, input.YearStart, input.YearEnd, input.UpdatedBy); err != nil {
		return nil, err
	}
	if existing, err := s.repository.FindByUniqueKey(ctx, input.BrandID, input.Model, input.YearStart, input.YearEnd); err == nil && existing.ID != id {
		return nil, mysqlInfra.ErrVehicleFitmentAlreadyExists
	} else if err != nil && !errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
		return nil, err
	}
	return s.repository.Update(ctx, id, input)
}

// FindOrCreateVehicleFitmentInput identifies the brand by BrandID or, when it
// is zero, by BrandName (created if missing).
type FindOrCreateVehicleFitmentInput struct {
	BrandID   int64
	BrandName string
	Model     string
	YearStart int
	YearEnd   *int
	ActorID   int64
}

// FindOrCreateFitment returns the fitment with the given brand/model/years,
// creating it only when none exists. created reports whether a new row was
// made. Brand and fitment are written in one transaction so a failed fitment
// doesn't leave a stray brand behind.
func (s *VehicleFitmentService) FindOrCreateFitment(ctx context.Context, input FindOrCreateVehicleFitmentInput) (*mysqlInfra.VehicleFitmentDTO, bool, error) {
	model := strings.TrimSpace(input.Model)
	if (input.BrandID <= 0 && strings.TrimSpace(input.BrandName) == "") ||
		validateVehicleFitment(1, model, input.YearStart, input.YearEnd, input.ActorID) != nil {
		return nil, false, ErrInvalidVehicleFitment
	}

	var fitment *mysqlInfra.VehicleFitmentDTO
	var created bool
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		brandsRepo := mysqlInfra.NewBrandsRepository(tx)
		fitmentsRepo := mysqlInfra.NewVehicleFitmentsRepository(tx)

		brandID, err := resolveBrandID(ctx, brandsRepo, input.BrandID, input.BrandName, input.ActorID)
		if err != nil {
			return err
		}

		existing, err := fitmentsRepo.FindByUniqueKey(ctx, brandID, model, input.YearStart, input.YearEnd)
		if err == nil {
			fitment = existing
			return nil
		}
		if !errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
			return err
		}

		fitment, err = fitmentsRepo.Create(ctx, mysqlInfra.CreateVehicleFitmentInput{
			BrandID:   brandID,
			Model:     model,
			YearStart: input.YearStart,
			YearEnd:   input.YearEnd,
			CreatedBy: input.ActorID,
		})
		created = err == nil
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return fitment, created, nil
}

// DeleteFitment soft-deletes a fitment. While products are linked to it the
// delete is refused (*InUseError) unless force is set, in which case the links
// are soft-deleted in the same transaction.
func (s *VehicleFitmentService) DeleteFitment(ctx context.Context, id int64, force bool, actorID int64) error {
	if id <= 0 || actorID <= 0 {
		return ErrInvalidVehicleFitment
	}

	fitment, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if fitment.ProductCount > 0 && !force {
		return &InUseError{Kind: "vehicle fitment", Count: fitment.ProductCount}
	}

	return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := mysqlInfra.NewVehicleFitmentsRepository(tx)
		if _, err := repo.SoftDeleteLinks(ctx, id, actorID); err != nil {
			return err
		}
		return repo.SoftDelete(ctx, id)
	})
}

func validateVehicleFitment(brandID int64, model string, yearStart int, yearEnd *int, actorID int64) error {
	if brandID <= 0 || strings.TrimSpace(model) == "" || actorID <= 0 {
		return ErrInvalidVehicleFitment
	}
	if yearStart < minFitmentYear || yearStart > maxFitmentYear {
		return ErrInvalidVehicleFitment
	}
	if yearEnd != nil && (*yearEnd < yearStart || *yearEnd > maxFitmentYear) {
		return ErrInvalidVehicleFitment
	}
	return nil
}

type BulkVehicleFitmentItem struct {
	Brand     string
	Model     string
	YearStart int
	YearEnd   *int
	SKU       string
	Motor     string
	Position  string
	Side      string
}

type BulkVehicleFitmentStatus string

const (
	BulkVehicleFitmentCreated BulkVehicleFitmentStatus = "created"
	BulkVehicleFitmentSkipped BulkVehicleFitmentStatus = "skipped"
	BulkVehicleFitmentError   BulkVehicleFitmentStatus = "error"
	BulkVehicleFitmentPending BulkVehicleFitmentStatus = "pending"
)

type BulkProductLinkResult struct {
	Status          BulkVehicleFitmentStatus `json:"status"`
	ProductID       int64                    `json:"productId,omitempty"`
	CompatibilityID int64                    `json:"compatibilityId,omitempty"`
	Error           string                   `json:"error,omitempty"`
}

type BulkVehicleFitmentResult struct {
	Index   int                           `json:"index"`
	Status  BulkVehicleFitmentStatus      `json:"status"`
	Fitment *mysqlInfra.VehicleFitmentDTO `json:"fitment,omitempty"`
	Error   string                        `json:"error,omitempty"`
	Product *BulkProductLinkResult        `json:"product,omitempty"`
}

// BulkImportFitments creates one vehicle fitment per item, resolving each
// brand by name (creating it if it doesn't exist yet) since bulk imports
// come from spreadsheets/ERPs that reference brands by name, not brandId.
// Items that collide with an existing fitment (same brand/model/year range)
// are reported as skipped rather than failing the whole batch. When an item
// carries a SKU, the matching product is linked to the fitment via
// ecom_product_vehicle_compatibility — the product's own brand_id (its
// manufacturer brand) is never read or modified, only its id. If no product
// has that SKU yet, the link is recorded in ecom_pending_product_vehicle_fitments
// and gets resolved automatically once a product with that SKU is created
// (see products.ProductService.CreateProduct).
//
// TODO(persistence): cada ítem escribe hasta en 3 tablas (ecom_brands,
// ecom_vehicle_fitments, ecom_product_vehicle_compatibility/ecom_pending_*);
// envolver el cuerpo del loop en mysqlInfra.WithinTx (linkProductIfRequested
// tendría que aceptar repos escopeados al tx). Hoy es recuperable porque cada
// paso maneja *AlreadyExists de forma idempotente.
func (s *VehicleFitmentService) BulkImportFitments(ctx context.Context, items []BulkVehicleFitmentItem, actorID int64) ([]BulkVehicleFitmentResult, error) {
	if actorID <= 0 {
		return nil, ErrInvalidVehicleFitment
	}

	results := make([]BulkVehicleFitmentResult, 0, len(items))
	for i, item := range items {
		brandName := strings.TrimSpace(item.Brand)
		model := strings.TrimSpace(item.Model)
		motor := strings.TrimSpace(item.Motor)
		position := strings.TrimSpace(item.Position)
		side := strings.TrimSpace(item.Side)

		if brandName == "" || model == "" || item.YearStart <= 0 || (item.YearEnd != nil && *item.YearEnd < item.YearStart) {
			results = append(results, BulkVehicleFitmentResult{Index: i, Status: BulkVehicleFitmentError, Error: ErrInvalidVehicleFitment.Error()})
			continue
		}

		brand, err := s.brandRepository.FindByName(ctx, brandName)
		if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
			brand, err = s.brandRepository.Create(ctx, mysqlInfra.CreateBrandInput{Name: brandName, CreatedBy: actorID})
		}
		if err != nil {
			results = append(results, BulkVehicleFitmentResult{Index: i, Status: BulkVehicleFitmentError, Error: err.Error()})
			continue
		}

		fitment, err := s.repository.Create(ctx, mysqlInfra.CreateVehicleFitmentInput{
			BrandID:   brand.ID,
			Model:     model,
			YearStart: item.YearStart,
			YearEnd:   item.YearEnd,
			CreatedBy: actorID,
		})
		if err != nil {
			if !errors.Is(err, mysqlInfra.ErrVehicleFitmentAlreadyExists) {
				results = append(results, BulkVehicleFitmentResult{Index: i, Status: BulkVehicleFitmentError, Error: err.Error()})
				continue
			}

			existing, findErr := s.repository.FindByUniqueKey(ctx, brand.ID, model, item.YearStart, item.YearEnd)
			if findErr != nil {
				results = append(results, BulkVehicleFitmentResult{Index: i, Status: BulkVehicleFitmentSkipped, Error: findErr.Error()})
				continue
			}

			results = append(results, BulkVehicleFitmentResult{
				Index:   i,
				Status:  BulkVehicleFitmentSkipped,
				Fitment: existing,
				Product: s.linkProductIfRequested(ctx, item.SKU, existing.ID, motor, position, side, actorID),
			})
			continue
		}

		results = append(results, BulkVehicleFitmentResult{
			Index:   i,
			Status:  BulkVehicleFitmentCreated,
			Fitment: fitment,
			Product: s.linkProductIfRequested(ctx, item.SKU, fitment.ID, motor, position, side, actorID),
		})
	}

	return results, nil
}

// linkProductIfRequested links the product matching sku to fitmentID, qualified
// by motor/position/side (MercadoLibre VIS fields: motor distinguishes engine
// variants of the same fitment; position/side describe how this specific
// product mounts on it — e.g. two different products, front-left vs
// front-right, can both target the same fitment). These three are part of the
// "same combination" the caller must not duplicate, so they flow into both the
// real compatibility row and the pending/staging row the same way.
func (s *VehicleFitmentService) linkProductIfRequested(ctx context.Context, sku string, fitmentID int64, motor, position, side string, actorID int64) *BulkProductLinkResult {
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil
	}

	product, err := s.productRepository.FindBySKU(ctx, sku)
	if err != nil {
		if !errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return &BulkProductLinkResult{Status: BulkVehicleFitmentError, Error: err.Error()}
		}

		_, pendingErr := s.pendingRepository.Create(ctx, mysqlInfra.CreatePendingProductVehicleFitmentInput{
			SKU:              sku,
			VehicleFitmentID: fitmentID,
			Motor:            motor,
			Position:         position,
			Side:             side,
			CreatedBy:        actorID,
		})
		if pendingErr != nil && !errors.Is(pendingErr, mysqlInfra.ErrPendingProductVehicleFitmentAlreadyExists) {
			return &BulkProductLinkResult{Status: BulkVehicleFitmentError, Error: pendingErr.Error()}
		}
		return &BulkProductLinkResult{Status: BulkVehicleFitmentPending}
	}

	compat, err := s.productVehicleCompatibilityRepository.Create(ctx, mysqlInfra.CreateProductVehicleCompatibilityInput{
		ProductID:        product.ID,
		VehicleFitmentID: fitmentID,
		Motor:            motor,
		Position:         position,
		Side:             side,
		CreatedBy:        actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductVehicleCompatibilityAlreadyExists) {
			return &BulkProductLinkResult{Status: BulkVehicleFitmentSkipped, ProductID: product.ID}
		}
		return &BulkProductLinkResult{Status: BulkVehicleFitmentError, ProductID: product.ID, Error: err.Error()}
	}

	return &BulkProductLinkResult{Status: BulkVehicleFitmentCreated, ProductID: product.ID, CompatibilityID: compat.ID}
}

type ResolvedPendingCompatibility struct {
	VehicleFitmentID int64 `json:"vehicleFitmentId"`
	CompatibilityID  int64 `json:"compatibilityId"`
}

type ResolvedPendingFitmentsBySKU struct {
	SKU             string                         `json:"sku"`
	Compatibilities []ResolvedPendingCompatibility `json:"compatibilities"`
}

// ResolvePendingFitments scans every unresolved row in
// ecom_pending_product_vehicle_fitments and, for each whose SKU now matches a
// product in ecom_products, creates the corresponding
// ecom_product_vehicle_compatibility row and marks the pending row resolved.
// Rows whose SKU still doesn't match any product are left pending for a
// future run. A row whose compatibility was already created some other way
// is still marked resolved (it's no longer waiting on anything) but is
// omitted from the response since it isn't a new compatibility. Per-row
// failures are logged and skipped rather than aborting the whole scan, so one
// bad row doesn't block the rest from resolving.
func (s *VehicleFitmentService) ResolvePendingFitments(ctx context.Context, actorID int64) ([]ResolvedPendingFitmentsBySKU, error) {
	if actorID <= 0 {
		return nil, ErrInvalidVehicleFitment
	}

	pending, err := s.pendingRepository.FindAllUnresolved(ctx)
	if err != nil {
		return nil, err
	}

	bySKU := make(map[string]*ResolvedPendingFitmentsBySKU)
	order := make([]string, 0)

	for _, item := range pending {
		product, err := s.productRepository.FindBySKU(ctx, item.SKU)
		if err != nil {
			if !errors.Is(err, mysqlInfra.ErrProductNotFound) {
				log.Printf("error looking up product for pending vehicle fitment %d (sku %s): %v", item.ID, item.SKU, err)
			}
			continue
		}

		// Crear la compatibilidad y marcar la fila pendiente como resuelta van
		// juntas en una transacción: si el MarkResolved falla, la compatibilidad
		// recién creada también se revierte y la próxima corrida reintenta
		// limpio, en vez de dejar una compat creada con la fila aún pendiente.
		var compatID int64
		var alreadyExisted bool
		txErr := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
			compatRepo := mysqlInfra.NewProductVehicleCompatibilityRepository(tx)
			pendingRepo := mysqlInfra.NewPendingProductVehicleFitmentsRepository(tx)

			compat, createErr := compatRepo.Create(ctx, mysqlInfra.CreateProductVehicleCompatibilityInput{
				ProductID:        product.ID,
				VehicleFitmentID: item.VehicleFitmentID,
				Motor:            item.Motor,
				Position:         item.Position,
				Side:             item.Side,
				CreatedBy:        actorID,
			})
			alreadyExisted = errors.Is(createErr, mysqlInfra.ErrProductVehicleCompatibilityAlreadyExists)
			if createErr != nil && !alreadyExisted {
				return createErr
			}
			if !alreadyExisted {
				compatID = compat.ID
			}

			return pendingRepo.MarkResolved(ctx, item.ID, product.ID, actorID)
		})
		if txErr != nil {
			log.Printf("error resolving pending vehicle fitment %d (sku %s): %v", item.ID, item.SKU, txErr)
			continue
		}

		if alreadyExisted {
			continue
		}

		group, ok := bySKU[item.SKU]
		if !ok {
			group = &ResolvedPendingFitmentsBySKU{SKU: item.SKU}
			bySKU[item.SKU] = group
			order = append(order, item.SKU)
		}
		group.Compatibilities = append(group.Compatibilities, ResolvedPendingCompatibility{
			VehicleFitmentID: item.VehicleFitmentID,
			CompatibilityID:  compatID,
		})
	}

	results := make([]ResolvedPendingFitmentsBySKU, 0, len(order))
	for _, sku := range order {
		results = append(results, *bySKU[sku])
	}

	return results, nil
}

type EquipmentFitmentService struct {
	db         *sql.DB
	repository *mysqlInfra.EquipmentFitmentsRepository
}

func NewEquipmentFitmentService(db *sql.DB, repository *mysqlInfra.EquipmentFitmentsRepository) *EquipmentFitmentService {
	return &EquipmentFitmentService{db: db, repository: repository}
}

func (s *EquipmentFitmentService) GetPaginatedFitments(ctx context.Context, filter mysqlInfra.EquipmentFitmentFilter, offset, pageSize int) (*mysqlInfra.PaginatedEquipmentFitments, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.Search(ctx, filter, offset, pageSize)
}

func (s *EquipmentFitmentService) GetFitmentByID(ctx context.Context, id int64) (*mysqlInfra.EquipmentFitmentDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidEquipmentFitment
	}
	return s.repository.FindByID(ctx, id)
}

// CreateFitment creates a fitment, failing with ErrEquipmentFitmentAlreadyExists
// when one with the same brand/type/model/serie exists (use FindOrCreateFitment
// to reuse it instead).
func (s *EquipmentFitmentService) CreateFitment(ctx context.Context, input mysqlInfra.CreateEquipmentFitmentInput) (*mysqlInfra.EquipmentFitmentDTO, error) {
	input.Model = normalizeOptional(input.Model)
	input.Serie = normalizeOptional(input.Serie)
	if err := validateEquipmentFitment(input.BrandID, input.EquipmentTypeID, input.Model, input.Serie, input.CreatedBy); err != nil {
		return nil, err
	}
	if _, err := s.repository.FindByUniqueKey(ctx, input.BrandID, input.EquipmentTypeID, input.Model, input.Serie); err == nil {
		return nil, ErrEquipmentFitmentAlreadyExists
	} else if !errors.Is(err, mysqlInfra.ErrEquipmentFitmentNotFound) {
		return nil, err
	}
	return s.repository.Create(ctx, input)
}

func (s *EquipmentFitmentService) UpdateFitment(ctx context.Context, id int64, input mysqlInfra.UpdateEquipmentFitmentInput) (*mysqlInfra.EquipmentFitmentDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidEquipmentFitment
	}
	input.Model = normalizeOptional(input.Model)
	input.Serie = normalizeOptional(input.Serie)
	if err := validateEquipmentFitment(input.BrandID, input.EquipmentTypeID, input.Model, input.Serie, input.UpdatedBy); err != nil {
		return nil, err
	}
	if existing, err := s.repository.FindByUniqueKey(ctx, input.BrandID, input.EquipmentTypeID, input.Model, input.Serie); err == nil && existing.ID != id {
		return nil, ErrEquipmentFitmentAlreadyExists
	} else if err != nil && !errors.Is(err, mysqlInfra.ErrEquipmentFitmentNotFound) {
		return nil, err
	}
	return s.repository.Update(ctx, id, input)
}

// FindOrCreateEquipmentFitmentInput identifies the brand by BrandID or
// BrandName and the type by EquipmentTypeID or EquipmentTypeName; a name that
// matches nothing is created.
type FindOrCreateEquipmentFitmentInput struct {
	BrandID           int64
	BrandName         string
	EquipmentTypeID   int64
	EquipmentTypeName string
	Model             *string
	Serie             *string
	ActorID           int64
}

// FindOrCreateFitment returns the fitment with the given brand/type/model/serie,
// creating it only when none exists. created reports whether a new row was
// made. Brand, type and fitment are written in one transaction.
func (s *EquipmentFitmentService) FindOrCreateFitment(ctx context.Context, input FindOrCreateEquipmentFitmentInput) (*mysqlInfra.EquipmentFitmentDTO, bool, error) {
	model := normalizeOptional(input.Model)
	serie := normalizeOptional(input.Serie)
	if (input.BrandID <= 0 && strings.TrimSpace(input.BrandName) == "") ||
		(input.EquipmentTypeID <= 0 && strings.TrimSpace(input.EquipmentTypeName) == "") ||
		validateEquipmentFitment(1, 1, model, serie, input.ActorID) != nil {
		return nil, false, ErrInvalidEquipmentFitment
	}

	var fitment *mysqlInfra.EquipmentFitmentDTO
	var created bool
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		brandsRepo := mysqlInfra.NewBrandsRepository(tx)
		typesRepo := mysqlInfra.NewEquipmentTypesRepository(tx)
		fitmentsRepo := mysqlInfra.NewEquipmentFitmentsRepository(tx)

		brandID, err := resolveBrandID(ctx, brandsRepo, input.BrandID, input.BrandName, input.ActorID)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrVehicleFitmentInvalidReference) {
				return mysqlInfra.ErrEquipmentFitmentInvalidReference
			}
			return err
		}

		typeID := input.EquipmentTypeID
		if typeID > 0 {
			if _, err := typesRepo.FindByID(ctx, typeID); err != nil {
				if errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
					return mysqlInfra.ErrEquipmentFitmentInvalidReference
				}
				return err
			}
		} else {
			name := strings.TrimSpace(input.EquipmentTypeName)
			equipmentType, err := typesRepo.FindByName(ctx, name)
			if errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
				equipmentType, err = typesRepo.Create(ctx, mysqlInfra.CreateEquipmentTypeInput{Name: name, CreatedBy: input.ActorID})
			}
			if err != nil {
				return err
			}
			typeID = equipmentType.ID
		}

		existing, err := fitmentsRepo.FindByUniqueKey(ctx, brandID, typeID, model, serie)
		if err == nil {
			fitment = existing
			return nil
		}
		if !errors.Is(err, mysqlInfra.ErrEquipmentFitmentNotFound) {
			return err
		}

		fitment, err = fitmentsRepo.Create(ctx, mysqlInfra.CreateEquipmentFitmentInput{
			BrandID:         brandID,
			EquipmentTypeID: typeID,
			Model:           model,
			Serie:           serie,
			CreatedBy:       input.ActorID,
		})
		created = err == nil
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return fitment, created, nil
}

// DeleteFitment soft-deletes a fitment. While products are linked to it the
// delete is refused (*InUseError) unless force is set, in which case the links
// are soft-deleted in the same transaction.
func (s *EquipmentFitmentService) DeleteFitment(ctx context.Context, id int64, force bool, actorID int64) error {
	if id <= 0 || actorID <= 0 {
		return ErrInvalidEquipmentFitment
	}

	fitment, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if fitment.ProductCount > 0 && !force {
		return &InUseError{Kind: "equipment fitment", Count: fitment.ProductCount}
	}

	return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := mysqlInfra.NewEquipmentFitmentsRepository(tx)
		if _, err := repo.SoftDeleteLinks(ctx, id, actorID); err != nil {
			return err
		}
		return repo.SoftDelete(ctx, id)
	})
}

// validateEquipmentFitment enforces that at least one of model/serie is
// present, since machinery compatibility is described by either one
// depending on the equipment family (e.g. montacargas por modelo,
// plataformas por serie).
func validateEquipmentFitment(brandID, equipmentTypeID int64, model, serie *string, actorID int64) error {
	if brandID <= 0 || equipmentTypeID <= 0 || actorID <= 0 {
		return ErrInvalidEquipmentFitment
	}

	hasModel := model != nil && strings.TrimSpace(*model) != ""
	hasSerie := serie != nil && strings.TrimSpace(*serie) != ""
	if !hasModel && !hasSerie {
		return ErrInvalidEquipmentFitment
	}

	return nil
}

type ProductVehicleCompatibilityService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductVehicleCompatibilityRepository
}

func NewProductVehicleCompatibilityService(db *sql.DB, repository *mysqlInfra.ProductVehicleCompatibilityRepository) *ProductVehicleCompatibilityService {
	return &ProductVehicleCompatibilityService{db: db, repository: repository}
}

func (s *ProductVehicleCompatibilityService) GetByProduct(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductVehicleCompatibilities, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductCompatibility
	}
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindByProductID(ctx, productID, offset, pageSize)
}

func (s *ProductVehicleCompatibilityService) Create(ctx context.Context, input mysqlInfra.CreateProductVehicleCompatibilityInput) (*mysqlInfra.ProductVehicleCompatibilityDTO, error) {
	if input.ProductID <= 0 || input.VehicleFitmentID <= 0 || input.CreatedBy <= 0 {
		return nil, ErrInvalidProductCompatibility
	}
	return s.repository.Create(ctx, input)
}

func (s *ProductVehicleCompatibilityService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidProductCompatibility
	}
	return s.repository.SoftDelete(ctx, id)
}

type ProductEquipmentCompatibilityService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductEquipmentCompatibilityRepository
}

func NewProductEquipmentCompatibilityService(db *sql.DB, repository *mysqlInfra.ProductEquipmentCompatibilityRepository) *ProductEquipmentCompatibilityService {
	return &ProductEquipmentCompatibilityService{db: db, repository: repository}
}

func (s *ProductEquipmentCompatibilityService) GetByProduct(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductEquipmentCompatibilities, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductCompatibility
	}
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindByProductID(ctx, productID, offset, pageSize)
}

func (s *ProductEquipmentCompatibilityService) Create(ctx context.Context, input mysqlInfra.CreateProductEquipmentCompatibilityInput) (*mysqlInfra.ProductEquipmentCompatibilityDTO, error) {
	if input.ProductID <= 0 || input.EquipmentFitmentID <= 0 || input.CreatedBy <= 0 {
		return nil, ErrInvalidProductCompatibility
	}
	return s.repository.Create(ctx, input)
}

func (s *ProductEquipmentCompatibilityService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidProductCompatibility
	}
	return s.repository.SoftDelete(ctx, id)
}
