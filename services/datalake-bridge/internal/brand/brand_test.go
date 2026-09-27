package brand

import "testing"

func TestResolve(t *testing.T) {
	cases := []struct{ code, findim, want string }{
		{"BOBCAT", "BOB", "Bobcat"},
		{"UNICARRIERS", "UCA", "Unicarriers Forklift"},
		{"", "UCA", "Unicarriers Forklift"}, // sin brandcode -> código financiero
		{"", "MCD", "Bobcat MH"},
		{"", "OTR", "Otras"},
		{"", "TDN", "Taylor-Dunn"},
		{"PPOW", "BOB", "Bobcat"}, // brandcode desconocido -> cae a código financiero
		{"", "", ""},              // nada -> sin marca
		{"", "XXX", ""},           // código desconocido -> sin marca
	}
	for _, c := range cases {
		if got := Resolve(c.code, c.findim); got != c.want {
			t.Errorf("Resolve(%q,%q)=%q, quiero %q", c.code, c.findim, got, c.want)
		}
	}
}
