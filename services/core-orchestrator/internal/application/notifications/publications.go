package notifications

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	// publicationWindow: un error sin eventos nuevos en este plazo se da por
	// cerrado. El refresco corre con la sincronización diaria del ERP, así que
	// un error que persiste vuelve a registrarse antes.
	publicationWindow = 72 * time.Hour
	// publicationQuiet: la tanda terminó si no hay eventos nuevos en este
	// plazo.
	publicationQuiet = 3 * time.Minute
	// publicationMaxWait: en una tanda larga se avisa lo acumulado sin esperar a
	// que termine.
	publicationMaxWait = 30 * time.Minute
	// publicationCheckEvery es cada cuánto se revisa la bitácora.
	publicationCheckEvery = 2 * time.Minute

	// publicationRowsInMessage: con más filas se adjunta un archivo.
	publicationRowsInMessage = 5
	publicationTopCauses     = 3
	// maxStoredCauses limita lo que se guarda por producto y conexión.
	maxStoredCauses = 20
)

type listingStateSource interface {
	LatestListingStates(ctx context.Context, window time.Duration, ignoredError string) ([]mysqlInfra.ListingStateDTO, error)
}

var volatilePatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), "<id>"},
	{regexp.MustCompile(`\bML[A-Z]\d{6,}\b`), "<publicación>"},
	{regexp.MustCompile(`\d{6,}`), "#"},
	{regexp.MustCompile(`\s+`), " "},
}

// normalizeCause quita lo que cambia de un intento a otro (identificadores), para
// reconocer el mismo error y agrupar las causas.
func normalizeCause(message string) string {
	text := sanitize(message)
	for _, volatile := range volatilePatterns {
		text = volatile.pattern.ReplaceAllString(text, volatile.replacement)
	}
	if text == "" {
		text = "error sin detalle"
	}
	return truncate(text, maxErrorRunes)
}

// publicationProblem reúne los errores vigentes de un producto en una conexión.
type publicationProblem struct {
	key                  string
	productID            int64
	connectionID         int64
	sku, product         string
	connection           string
	causes               []string // normalizadas, sin repetir
	messages             []string // como se muestran
	actions              []string
	newestAge, oldestAge time.Duration
}

var listingActionLabels = map[string]string{
	mysqlInfra.ListingEventPublish: "publicar",
	mysqlInfra.ListingEventRefresh: "actualizar precio/stock",
	mysqlInfra.ListingEventResync:  "resincronizar",
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// groupProblems junta, por producto y conexión, las publicaciones cuyo último
// evento es un error. Respeta el orden de llegada.
func groupProblems(states []mysqlInfra.ListingStateDTO) []*publicationProblem {
	byKey := map[string]*publicationProblem{}
	ordered := make([]*publicationProblem, 0)
	for _, state := range states {
		if state.Outcome != mysqlInfra.ListingEventError {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d", incidentPublicationError, state.ProductID, state.ConnectionID)
		problem := byKey[key]
		if problem == nil {
			problem = &publicationProblem{
				key: key, productID: state.ProductID, connectionID: state.ConnectionID,
				sku: state.SKU, product: state.ProductName, connection: state.ConnectionName,
				newestAge: state.Age, oldestAge: state.Age,
			}
			byKey[key] = problem
			ordered = append(ordered, problem)
		}
		problem.newestAge = min(problem.newestAge, state.Age)
		problem.oldestAge = max(problem.oldestAge, state.Age)
		if len(problem.causes) < maxStoredCauses {
			problem.causes = appendUnique(problem.causes, normalizeCause(state.Message))
		}
		problem.messages = appendUnique(problem.messages, shortError(state.Message))
		action := listingActionLabels[state.Action]
		if action == "" {
			action = state.Action
		}
		problem.actions = appendUnique(problem.actions, action)
	}
	return ordered
}

// hasNewCause: solo se avisa de un error que no se hubiera visto; que
// desaparezca uno de varios no es novedad.
func hasNewCause(previous string, causes []string) bool {
	known := map[string]bool{}
	for _, line := range strings.Split(previous, "\n") {
		known[line] = true
	}
	for _, cause := range causes {
		if !known[cause] {
			return true
		}
	}
	return false
}

// checkPublicationErrors compara los errores vigentes de la bitácora con los
// incidentes abiertos. Cuando la tanda termina, envía un solo resumen con los
// productos con errores nuevos; los que ya no fallan se cierran sin aviso.
func (s *Service) checkPublicationErrors(ctx context.Context) error {
	states, err := s.listingStates.LatestListingStates(ctx, publicationWindow, channelListingsApp.ReasonNoStock)
	if err != nil {
		return err
	}
	open, err := s.incidents.ListOpenByType(ctx, incidentPublicationError)
	if err != nil {
		return err
	}
	openByKey := make(map[string]mysqlInfra.NotificationIncidentDTO, len(open))
	for _, incident := range open {
		openByKey[incident.IncidentKey] = incident
	}

	problems := groupProblems(states)
	current := make(map[string]bool, len(problems))
	fresh := make([]*publicationProblem, 0)
	var failures []error
	for _, problem := range problems {
		current[problem.key] = true
		incident, isOpen := openByKey[problem.key]
		if !isOpen || hasNewCause(incident.Message, problem.causes) {
			fresh = append(fresh, problem)
		}
	}

	for _, incident := range open {
		if !current[incident.IncidentKey] {
			if err := s.resolveIncident(ctx, incident.IncidentKey, nil); err != nil {
				failures = append(failures, err)
			}
		}
	}

	if len(fresh) > 0 && batchSettled(states, fresh) {
		if err := s.reportPublicationProblems(ctx, fresh); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func batchSettled(states []mysqlInfra.ListingStateDTO, fresh []*publicationProblem) bool {
	quiet := true
	for _, state := range states {
		if state.Age < publicationQuiet {
			quiet = false
			break
		}
	}
	if quiet {
		return true
	}
	for _, problem := range fresh {
		if problem.oldestAge >= publicationMaxWait {
			return true
		}
	}
	return false
}

// reportPublicationProblems abre o actualiza un incidente por producto y
// conexión y encola un único resumen, todo en una transacción.
func (s *Service) reportPublicationProblems(ctx context.Context, fresh []*publicationProblem) error {
	n := publicationNotice(fresh)

	return s.inUnit(ctx, func(tx unit) error {
		// Igual que en openIncident: sin destinatarios no se marca como avisado.
		users, err := s.audience(ctx, tx, n)
		if err != nil || len(users) == 0 {
			return err
		}

		now := s.now()
		keys := make([]string, 0, len(fresh))
		for _, problem := range fresh {
			detail := strings.Join(problem.causes, "\n")
			signature := signatureOf(problem.causes)
			keys = append(keys, problem.key+"="+signature)

			incident, err := tx.FindOpenIncident(ctx, problem.key)
			if err != nil {
				return err
			}
			if incident != nil {
				if err := tx.TouchIncident(ctx, incident.ID, signature, detail, now); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.OpenIncident(ctx, mysqlInfra.OpenNotificationIncidentInput{
				IncidentType: incidentPublicationError,
				IncidentKey:  problem.key,
				Signature:    signature,
				ConnectionID: &problem.connectionID,
				ProductID:    &problem.productID,
				Message:      detail,
				At:           now,
			}); err != nil {
				return err
			}
		}

		dedupeKey := fmt.Sprintf("publication:%.32s:%d", signatureOf(keys), now.UnixNano())
		return s.enqueue(ctx, tx, dedupeKey, n)
	})
}

// publicationNotice resume los problemas: totales, causas principales y el
// detalle en el texto o en un archivo.
func publicationNotice(problems []*publicationProblem) notice {
	products := map[int64]bool{}
	causeCount := map[string]int{}
	causes := make([]string, 0)
	rows := make([][]string, 0, len(problems))
	for _, problem := range problems {
		products[problem.productID] = true
		for _, cause := range problem.causes {
			if causeCount[cause] == 0 {
				causes = append(causes, cause)
			}
			causeCount[cause]++
		}
		rows = append(rows, []string{
			problem.sku, problem.product, problem.connection,
			strings.Join(problem.actions, ", "), strings.Join(problem.messages, " | "),
		})
	}
	sort.SliceStable(causes, func(i, j int) bool { return causeCount[causes[i]] > causeCount[causes[j]] })

	var text strings.Builder
	fmt.Fprintf(&text, "🟠 Errores al publicar o resincronizar (nuevos o distintos)\nProductos: %d · Casos por conexión: %d",
		len(products), len(problems))

	n := notice{notificationType: TypePublicationErrors}
	if len(problems) <= publicationRowsInMessage {
		for _, problem := range problems {
			fmt.Fprintf(&text, "\n\n• %s · %s\n%s", problem.sku, problem.connection,
				truncate(strings.Join(problem.messages, " | "), maxErrorRunes))
		}
	} else {
		text.WriteString("\n\nCausas principales:")
		for i, cause := range causes {
			if i == publicationTopCauses {
				break
			}
			fmt.Fprintf(&text, "\n• (%d) %s", causeCount[cause], truncate(cause, 180))
		}
		text.WriteString("\n\nEl detalle va en el archivo.")
		n.attachmentName = "errores-publicacion.csv"
		n.attachment = buildCSV([]string{"sku", "producto", "conexion", "accion", "error"}, rows)
	}
	n.text = text.String()
	return n
}
