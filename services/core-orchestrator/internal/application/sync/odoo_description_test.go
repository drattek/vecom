package sync

import "testing"

func TestOdooDescriptionHTML(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"Hola\nmundo", "<p>Hola<br>mundo</p>"},
		{"Uno\r\n\r\nDos", "<p>Uno</p><p>Dos</p>"},
		{"Compatible con:\n\nRetro Bobcat\n\n-B730M\n\n-B900\n\n-B760\n\nImportante: x", "<p>Compatible con:</p><p>Retro Bobcat</p><ul><li>B730M</li><li>B900</li><li>B760</li></ul><p>Importante: x</p>"},
		{"- a\n- b", "<ul><li>a</li><li>b</li></ul>"},
		{"a < b & c", "<p>a &lt; b &amp; c</p>"},
		{"<p>Ya es HTML</p>", "<p>Ya es HTML</p>"},
	}
	for _, c := range cases {
		if got := odooDescriptionHTML(c.in); got != c.want {
			t.Errorf("odooDescriptionHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
