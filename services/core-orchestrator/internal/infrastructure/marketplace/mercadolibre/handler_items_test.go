package mercadolibre

import (
	"encoding/json"
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
