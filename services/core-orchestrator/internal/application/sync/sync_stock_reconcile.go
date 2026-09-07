package sync

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	mysqlRepo "core-orchestrator/internal/infrastructure/mysql"
)

// stockReconcileBatchSize es cuántas bajas a 0 se agrupan por transacción MySQL,
// mismo criterio que nissanSyncBatchSize: cada baja son 2 escrituras (UPDATE de
// ecom_product_stock + INSERT en ecom_stock_movements) y compartir el COMMIT entre
// un lote evita que el fsync por registro domine el tiempo.
const stockReconcileBatchSize = 250

// stockReconcileMaxZeroRatio es el umbral de seguridad por almacén: si en una
// corrida más de este porcentaje de las posiciones con stock > 0 de un almacén
// (visto) intentaran bajar a 0, se asume que el feed de esa corrida llegó
// incompleto/truncado y se omite bajar a 0 ese almacén entero. El stock real se
// mueve mucho, pero que >50% de un almacén se vacíe de un día para el otro es más
// probable que sea un problema de la fuente que un vaciado real.
const stockReconcileMaxZeroRatio = 0.5

// stockKey identifica una fila de existencia local por (producto, sucursal, almacén),
// la misma granularidad con la que el ERP/Nissan reportan stock.
type stockKey struct {
	productID   int64
	branchID    int64
	warehouseID int64
}

// stockSeen acumula, durante una corrida de sync de stock, qué se vio en el feed
// de la fuente. El feed viene filtrado a stock > 0 (RE_VEXISTENCIAS.RELA_EXISTENCIAACTUAL
// > 0 / dyn.ItemInventLocation.Disponible > 0), así que "una fila no apareció" no
// distingue entre "su stock bajó a 0" y "ese almacén entero no vino en esta corrida".
// Por eso se registra por separado el set de almacenes vistos: la reconciliación
// solo confía en los ceros por ausencia dentro de un almacén que apareció al menos
// una vez en la corrida.
type stockSeen struct {
	keys       map[stockKey]struct{}
	warehouses map[int64]struct{}
}

func newStockSeen() *stockSeen {
	return &stockSeen{
		keys:       make(map[stockKey]struct{}),
		warehouses: make(map[int64]struct{}),
	}
}

// mark registra que (productID, branchID, warehouseID) vino en el feed de esta
// corrida. Se llama para cada registro leído aunque su escritura en MySQL falle:
// la fuente lo reportó con stock > 0, así que no debe bajarse a 0 por ausencia.
func (s *stockSeen) mark(productID, branchID, warehouseID int64) {
	s.keys[stockKey{productID, branchID, warehouseID}] = struct{}{}
	s.warehouses[warehouseID] = struct{}{}
}

func (s *stockSeen) sawWarehouse(warehouseID int64) bool {
	_, ok := s.warehouses[warehouseID]
	return ok
}

func (s *stockSeen) sawStock(productID, branchID, warehouseID int64) bool {
	_, ok := s.keys[stockKey{productID, branchID, warehouseID}]
	return ok
}

// reconcileZeroedStock baja a 0 el stock local de la fuente sourceID que quedó
// "colgado" en un valor > 0 porque su fila desapareció del feed. Es la contraparte
// del filtro stock > 0 que la fuente aplica para no leer decenas de miles de filas
// en 0: sin esto, un almacén que hoy tiene 1 y mañana 0 nunca se actualiza porque
// mañana simplemente no viene.
//
// Reglas (ver stockSeen):
//   - solo toca filas cuyo almacén apareció al menos una vez en la corrida; si un
//     almacén entero no vino, se asume falta de cobertura de esa corrida, no un
//     vaciado real, y se saltea.
//   - umbral de seguridad por almacén (stockReconcileMaxZeroRatio): si en un
//     almacén más del 50% de sus posiciones con stock > 0 intentaran bajar a 0, se
//     omite bajar a 0 ese almacén entero (feed probablemente truncado).
//   - cada baja genera su movimiento en ecom_stock_movements (movement_type "sync").
//
// Es idempotente: las filas ya en 0 no vuelven a salir de FindPositiveBySource, así
// que un reintento del consumer no genera movimientos duplicados. Un error se loguea
// y no se propaga (mismo criterio que refreshListingsAfter*Sync): devolver error
// haría que RabbitMQ reencole y reprocese todo el sync, y la próxima corrida vuelve
// a intentar la reconciliación de todos modos.
func (s *SyncService) reconcileZeroedStock(ctx context.Context, sourceID int64, seen *stockSeen) {
	stockRepo := mysqlRepo.NewProductStockRepository(s.db)

	rows, err := stockRepo.FindPositiveBySource(ctx, sourceID)
	if err != nil {
		log.Printf("Stock reconcile (source=%d): no se pudo listar el stock positivo: %v", sourceID, err)
		return
	}

	// Se agrupan los candidatos por almacén para poder aplicar el umbral por almacén
	// (stockReconcileMaxZeroRatio) antes de bajar nada a 0: positiveCount es cuántas
	// posiciones con stock > 0 tiene el almacén (vistas o no), candidates son las que
	// no vinieron en la corrida.
	type warehouseZeroing struct {
		positiveCount int
		candidates    []mysqlRepo.PositiveStockRow
	}

	perWarehouse := make(map[int64]*warehouseZeroing)
	skippedWarehouseAbsent := 0
	for _, row := range rows {
		if !seen.sawWarehouse(row.WarehouseID) {
			skippedWarehouseAbsent++
			continue
		}

		wz := perWarehouse[row.WarehouseID]
		if wz == nil {
			wz = &warehouseZeroing{}
			perWarehouse[row.WarehouseID] = wz
		}
		wz.positiveCount++

		if seen.sawStock(row.ProductID, row.BranchID, row.WarehouseID) {
			continue
		}
		wz.candidates = append(wz.candidates, row)
	}

	toZero := make([]mysqlRepo.PositiveStockRow, 0)
	skippedThreshold := 0
	for warehouseID, wz := range perWarehouse {
		if len(wz.candidates) == 0 {
			continue
		}

		// "más del 50%": len(candidates) > positiveCount * ratio. Con ratio 0.5 y
		// aritmética entera equivale a 2*candidates > positiveCount.
		if float64(len(wz.candidates)) > float64(wz.positiveCount)*stockReconcileMaxZeroRatio {
			log.Printf(
				"Stock reconcile (source=%d): almacén %d omitido — %d/%d posiciones (>%.0f%%) intentaban bajar a 0 (feed probablemente incompleto)",
				sourceID, warehouseID, len(wz.candidates), wz.positiveCount, stockReconcileMaxZeroRatio*100,
			)
			skippedThreshold += len(wz.candidates)
			continue
		}

		toZero = append(toZero, wz.candidates...)
	}

	log.Printf(
		"Stock reconcile (source=%d): %d filas con stock>0, %d a bajar a 0, %d saltadas (almacén ausente), %d saltadas (umbral >%.0f%% por almacén)",
		sourceID, len(rows), len(toZero), skippedWarehouseAbsent, skippedThreshold, stockReconcileMaxZeroRatio*100,
	)

	if len(toZero) == 0 {
		return
	}

	zeroed := 0
	for start := 0; start < len(toZero); start += stockReconcileBatchSize {
		end := min(start+stockReconcileBatchSize, len(toZero))
		batch := toZero[start:end]

		txErr := mysqlRepo.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
			stockRepo := mysqlRepo.NewProductStockRepository(tx)
			movementsRepo := mysqlRepo.NewStockMovementsRepository(tx)

			for _, row := range batch {
				if _, err := stockRepo.Update(ctx, row.ID, mysqlRepo.UpdateProductStockInput{
					AvailableQty: 0,
					UpdatedBy:    systemUserID,
				}); err != nil {
					return fmt.Errorf("zeroing stock row %d: %w", row.ID, err)
				}

				if _, err := movementsRepo.Create(ctx, mysqlRepo.CreateStockMovementInput{
					ProductID:      row.ProductID,
					BranchID:       row.BranchID,
					WarehouseID:    row.WarehouseID,
					MovementType:   "sync",
					QuantityBefore: row.AvailableQty,
					QuantityChange: -row.AvailableQty,
					QuantityAfter:  0,
					UpdatedBy:      systemUserID,
				}); err != nil {
					return fmt.Errorf("recording zeroing movement for stock row %d: %w", row.ID, err)
				}
			}

			return nil
		})
		if txErr != nil {
			log.Printf("Stock reconcile (source=%d): lote [%d:%d] falló: %v", sourceID, start, end, txErr)
			continue
		}

		zeroed += len(batch)
	}

	log.Printf("Stock reconcile (source=%d): %d/%d filas bajadas a 0", sourceID, zeroed, len(toZero))
}
