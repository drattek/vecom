package mercadolibre

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GET /items devuelve las imágenes como {id, url, secure_url} sin "source";
// DownloadURL debe resolverlas igual que a las construidas localmente.
func TestItemPictureDownloadURL(t *testing.T) {
	var item ItemDetail
	payload := `{"pictures":[
		{"id":"1","url":"http://http2.mlstatic.com/a.jpg","secure_url":"https://http2.mlstatic.com/a.jpg"},
		{"id":"2","url":"http://http2.mlstatic.com/b.jpg"},
		{"source":"https://example.com/c.jpg"},
		{}
	]}`
	if err := json.Unmarshal([]byte(payload), &item); err != nil {
		t.Fatal(err)
	}

	want := []string{"https://http2.mlstatic.com/a.jpg", "http://http2.mlstatic.com/b.jpg", "https://example.com/c.jpg", ""}
	for i, picture := range item.Pictures {
		if got := picture.DownloadURL(); got != want[i] {
			t.Errorf("picture %d: DownloadURL() = %q, esperaba %q", i, got, want[i])
		}
	}
}

// El payload de creación sigue enviando solo "source".
func TestItemPictureCreatePayload(t *testing.T) {
	out, err := json.Marshal(ItemPicture{Source: "https://example.com/c.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"source":"https://example.com/c.jpg"}` {
		t.Errorf("payload inesperado: %s", out)
	}
}

// Un ítem con family_name rechaza "title" (BODY_INVALID_FIELDS): su título se
// cambia con family_name. Los demás ítems siguen usando "title".
func TestUpdateItemTitleField(t *testing.T) {
	tests := []struct {
		name          string
		useFamilyName bool
		want          string
	}{
		{"ítem con family_name", true, `{"family_name":"Tolva Original Nissan"}`},
		{"ítem sin family_name", false, `{"title":"Tolva Original Nissan"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				gotMethod, gotBody = r.Method, string(body)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			handler := NewItemsHandler(NewClient(server.Client(), server.URL, nil))
			err := handler.UpdateItemTitle(context.Background(), UpdateItemTitleRequest{
				AccessToken:   "token",
				ExternalID:    "MLM1",
				Title:         "Tolva Original Nissan",
				UseFamilyName: tt.useFamilyName,
			})
			if err != nil {
				t.Fatal(err)
			}
			if gotMethod != http.MethodPut || gotBody != tt.want {
				t.Errorf("%s %s, esperaba PUT %s", gotMethod, gotBody, tt.want)
			}
		})
	}
}

// El cambio de tipo de publicación usa la ruta documentada (/marketplace/...) y
// cae a la antigua si esa responde 404.
func TestUpdateItemListingTypeFallsBackOnNotFound(t *testing.T) {
	var paths []string
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		paths = append(paths, r.Method+" "+r.URL.Path)
		bodies = append(bodies, string(body))
		if r.URL.Path == "/marketplace/items/MLM1/listing_type" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found","status":404}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	handler := NewItemsHandler(NewClient(server.Client(), server.URL, nil))
	if err := handler.UpdateItemListingType(context.Background(), "token", "MLM1", "gold_pro"); err != nil {
		t.Fatal(err)
	}

	wantPaths := []string{"POST /marketplace/items/MLM1/listing_type", "POST /items/MLM1/listing_type"}
	if len(paths) != 2 || paths[0] != wantPaths[0] || paths[1] != wantPaths[1] {
		t.Errorf("rutas = %v, esperaba %v", paths, wantPaths)
	}
	for _, body := range bodies {
		if body != `{"id":"gold_pro"}` {
			t.Errorf("cuerpo = %s", body)
		}
	}
}

func TestUpdateItemListingTypeStopsOnOtherErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"listing type already changed","status":400}`))
	}))
	defer server.Close()

	handler := NewItemsHandler(NewClient(server.Client(), server.URL, nil))
	if err := handler.UpdateItemListingType(context.Background(), "token", "MLM1", "gold_pro"); err == nil {
		t.Fatal("esperaba el error de MercadoLibre")
	}
	if calls != 1 {
		t.Errorf("un 400 no debe reintentar en otra ruta: %d llamadas", calls)
	}
}

// Sacar un ítem de la tienda oficial envía un null explícito, no omite el campo.
func TestRemoveItemOfficialStoreSendsExplicitNull(t *testing.T) {
	var method, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method, body = r.Method, string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	handler := NewItemsHandler(NewClient(server.Client(), server.URL, nil))
	if err := handler.RemoveItemOfficialStore(context.Background(), "token", "MLM1"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || body != `{"official_store_id":null}` {
		t.Errorf("%s %s", method, body)
	}
}
