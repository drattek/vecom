package api

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"
)

// recoverJSON es el equivalente a chi middleware.Recoverer pero devolviendo el
// mismo shape JSON que el resto de la API ({"error": ...}) en vez del HTML por
// defecto de chi. Un panic en un handler queda contenido en esa request: se
// loguea con request id y stack, y el cliente recibe un 500 limpio.
//
// net/http ya recupera panics por conexión, así que esto no evita que el
// proceso muera (no lo hacía) — su valor es logging uniforme y una respuesta
// consistente para el front.
func recoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}

			// http.ErrAbortHandler es la forma idiomática de abortar una
			// respuesta a propósito: se re-propaga para que net/http la maneje.
			if rvr == http.ErrAbortHandler {
				panic(rvr)
			}

			reqID := middleware.GetReqID(r.Context())
			log.Printf("panic recuperado en %s %s (request_id=%s): %v\n%s", r.Method, r.URL.Path, reqID, rvr, debug.Stack())

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		}()

		next.ServeHTTP(w, r)
	})
}
