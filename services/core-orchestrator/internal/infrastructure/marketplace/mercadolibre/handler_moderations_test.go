package mercadolibre

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newModerationsHandler(t *testing.T, status int, body string) *ModerationsHandler {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewModerationsHandler(NewClient(server.Client(), server.URL, nil))
}

func TestLastModeration(t *testing.T) {
	// Forma documentada por MercadoLibre.
	const documented = `[{"name":"POOR_QUALITY_THUMBNAIL","id":"1","wordings":[
		{"type":"REMEDY","value":"Corrige tus fotos."},{"type":"REASON","value":"La portada no tiene fondo blanco."}]}]`

	for name, body := range map[string]string{
		"list":   documented,
		"object": documented[1 : len(documented)-1],
	} {
		t.Run(name, func(t *testing.T) {
			got, err := newModerationsHandler(t, 200, body).LastModeration(context.Background(), "token", "MLM1")
			if err != nil || got == nil {
				t.Fatalf("got %v, err %v", got, err)
			}
			if got.Reason != "La portada no tiene fondo blanco." || got.Remedy != "Corrige tus fotos." {
				t.Fatalf("unexpected moderation: %+v", got)
			}
		})
	}
}

func TestLastModerationWithoutModeration(t *testing.T) {
	for name, handler := range map[string]*ModerationsHandler{
		"not found":  newModerationsHandler(t, 404, `{"message":"not found"}`),
		"empty list": newModerationsHandler(t, 200, `[]`),
	} {
		got, err := handler.LastModeration(context.Background(), "token", "MLM1")
		if err != nil || got != nil {
			t.Errorf("%s: got %v, err %v", name, got, err)
		}
	}

	if _, err := newModerationsHandler(t, 500, `{}`).LastModeration(context.Background(), "token", "MLM1"); err == nil {
		t.Error("a server error must be reported so it is retried")
	}
}

func TestLastInfractionOnlyMatchesTheItem(t *testing.T) {
	const body = `{"infractions":[{"related_item_id":"MLM1","reason":"Denunciada.","remedy":""}]}`
	handler := newModerationsHandler(t, 200, body)

	got, err := handler.LastInfraction(context.Background(), "token", 7, "MLM1")
	if err != nil || got == nil || got.Reason != "Denunciada." {
		t.Fatalf("got %v, err %v", got, err)
	}
	other, err := handler.LastInfraction(context.Background(), "token", 7, "MLM2")
	if err != nil || other != nil {
		t.Fatalf("got %v, err %v", other, err)
	}
}
