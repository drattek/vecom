package domain

type Inventory struct {
	Code  string  `json:"articulo"`
	Stock float64 `json:"disponible"`
	Cost  float64 `json:"costoPromedio"`
}
