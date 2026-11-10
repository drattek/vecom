package product_image_import

import "testing"

func TestParseYouTubeURL(t *testing.T) {
	const want = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	valid := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?si=abc",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ",
		"https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		"https://www.youtube.com/live/dQw4w9WgXcQ",
		"  https://youtu.be/dQw4w9WgXcQ  ",
	}
	for _, raw := range valid {
		got, isYT, err := ParseYouTubeURL(raw)
		if !isYT || err != nil || got != want {
			t.Errorf("%q → (%q, %v, %v), want (%q, true, nil)", raw, got, isYT, err, want)
		}
	}

	// Es de YouTube pero sin ID válido: error, no se trata como archivo.
	for _, raw := range []string{
		"https://www.youtube.com/",
		"https://www.youtube.com/watch?v=corto",
		"https://youtu.be/",
		"https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxx",
	} {
		if _, isYT, err := ParseYouTubeURL(raw); !isYT || err == nil {
			t.Errorf("%q → isYouTube=%v err=%v, want true + error", raw, isYT, err)
		}
	}

	// Cualquier otra cosa es un archivo de video normal.
	for _, raw := range []string{
		"https://cdn.ejemplo.com/video.mp4",
		"https://notyoutube.com/watch?v=dQw4w9WgXcQ",
		"https://vimeo.com/123456",
	} {
		if got, isYT, err := ParseYouTubeURL(raw); isYT || err != nil || got != "" {
			t.Errorf("%q → (%q, %v, %v), want (\"\", false, nil)", raw, got, isYT, err)
		}
	}
}
