package sync

import "testing"

func TestCapacityFromName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"MONTACARGAS COMBUSTION LP GAS 5000 LBS", "5000 LBS"},
		{"FORKLIFT 2.5 Tons LPG", "2.5 Ton"},
		{"MONTACARGAS DOBLE-COMBUSTION 2500 KGS", "2500 KG"},
		{"TRACTOR DE ARRASTRE TGX20B PARA 10000 LBS", "10000 LBS"},
		{"PLATAFORMA DE TIJERA 32 PIES", ""}, // pies no es capacidad
		{"COMPRESOS DE 185CFM PA185VKUB", ""},
	}
	for _, c := range cases {
		if got := capacityFromName(c.name); got != c.want {
			t.Errorf("capacityFromName(%q)=%q, quiero %q", c.name, got, c.want)
		}
	}
}
