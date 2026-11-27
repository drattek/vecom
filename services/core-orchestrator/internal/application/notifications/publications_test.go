package notifications

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func listingError(productID, connectionID int64, message string, age time.Duration) mysqlInfra.ListingStateDTO {
	return mysqlInfra.ListingStateDTO{
		ProductID: productID, SKU: fmt.Sprintf("SKU-%d", productID), ProductName: "Producto",
		ConnectionID: connectionID, ConnectionName: fmt.Sprintf("Cuenta %d", connectionID),
		Action: mysqlInfra.ListingEventResync, Outcome: mysqlInfra.ListingEventError,
		Message: message, Age: age,
	}
}

func listingSuccess(productID, connectionID int64, age time.Duration) mysqlInfra.ListingStateDTO {
	state := listingError(productID, connectionID, "ok", age)
	state.Outcome = mysqlInfra.ListingEventSuccess
	return state
}

func checkPublications(t *testing.T, service *Service) {
	t.Helper()
	if err := service.checkPublicationErrors(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPublicationErrorsWaitForTheBatchToEnd(t *testing.T) {
	service, store, _ := newEventService()

	store.states = []mysqlInfra.ListingStateDTO{
		listingError(1, 1, "falta la marca", 10*time.Minute),
		listingSuccess(2, 1, time.Minute), // la tanda sigue
	}
	checkPublications(t, service)
	assertMessages(t, store)

	store.states[1].Age = 4 * time.Minute
	checkPublications(t, service)
	assertMessages(t, store, "Productos: 1 · Casos por conexión: 1")
}

func TestPublicationErrorsDoNotWaitForeverOnLongBatches(t *testing.T) {
	service, store, _ := newEventService()
	store.states = []mysqlInfra.ListingStateDTO{
		listingError(1, 1, "falta la marca", 31*time.Minute),
		listingSuccess(2, 1, 5*time.Second),
	}

	checkPublications(t, service)

	assertMessages(t, store, "Productos: 1")
}

func TestPublicationErrorsOnlyReportNewOrDifferent(t *testing.T) {
	service, store, _ := newEventService()
	run := func(states ...mysqlInfra.ListingStateDTO) {
		store.states = states
		checkPublications(t, service)
	}
	const old = 10 * time.Minute

	run(listingError(1, 1, "precio inválido en MLM123456789", old))
	// El mismo error con otro identificador de publicación no es nuevo.
	run(listingError(1, 1, "precio inválido en MLM987654321", old))
	// Dos errores; después se arregla uno: eso no es novedad.
	run(listingError(1, 1, "precio inválido en MLM123456789", old), listingError(1, 1, "falta GTIN", old))
	run(listingError(1, 1, "falta GTIN", old))
	// Se corrige y más tarde vuelve a fallar: sí se avisa.
	run(listingSuccess(1, 1, old))
	if len(store.open) != 0 {
		t.Fatalf("the incident must be closed: %v", store.open)
	}
	run(listingError(1, 1, "falta GTIN", old))

	assertMessages(t, store, "precio inválido", "falta GTIN", "falta GTIN")
}

func TestPublicationErrorsSummary(t *testing.T) {
	service, store, _ := newEventService()
	for product := int64(1); product <= 4; product++ {
		store.states = append(store.states,
			listingError(product, 1, "token rechazado", 10*time.Minute),
			listingError(product, 2, "falta la categoría", 10*time.Minute))
	}
	store.states = append(store.states, listingError(5, 1, "token rechazado", 10*time.Minute))

	checkPublications(t, service)

	assertMessages(t, store, "Productos: 5 · Casos por conexión: 9")
	delivery := store.deliveries[0]
	if !strings.Contains(delivery.Message, "(5) token rechazado") || !strings.Contains(delivery.Message, "(4) falta la categoría") {
		t.Errorf("main causes missing: %q", delivery.Message)
	}
	if strings.Index(delivery.Message, "(5)") > strings.Index(delivery.Message, "(4)") {
		t.Errorf("causes must be sorted by frequency: %q", delivery.Message)
	}
	if delivery.AttachmentName != "errores-publicacion.csv" {
		t.Fatalf("attachment = %q", delivery.AttachmentName)
	}
	if lines := strings.Count(string(delivery.AttachmentContent), "\r\n"); lines != 10 {
		t.Errorf("csv lines = %d, want header + 9", lines)
	}
	if len(store.open) != 9 {
		t.Errorf("open incidents = %d", len(store.open))
	}

	checkPublications(t, service)
	if len(store.deliveries) != 1 {
		t.Errorf("must not repeat the summary")
	}
}

func TestPublicationErrorsWaitForARecipient(t *testing.T) {
	service, store, _ := newEventService()
	admin := store.recipients
	store.states = []mysqlInfra.ListingStateDTO{listingError(1, 1, "falta la marca", 10*time.Minute)}

	store.recipients = nil
	checkPublications(t, service)
	store.recipients = admin
	checkPublications(t, service)

	assertMessages(t, store, "falta la marca")
}
