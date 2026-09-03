package domain

// Inventory refleja ItemInventLocationDTO (synapse-bridge/dto/ItemInventLocationDTO.java): los
// json tags deben coincidir exactamente con los nombres de componente del record Java, igual que
// en existencia.go, porque Jackson serializa por nombre de componente sin transformaciones.
type Inventory struct {
	Source          string  `json:"source"`
	Code            string  `json:"articulo"`
	Description     string  `json:"descripcion"`
	PartNumber      string  `json:"numeroParte"`
	Branch          string  `json:"sucursal"`
	Warehouse       string  `json:"almacen"`
	BranchName      string  `json:"nombreSucursal"`
	Group           string  `json:"grupo"`
	Stock           float64 `json:"disponible"`
	TransactionCost float64 `json:"costoTransaccion"`
	Cost            float64 `json:"costoPromedio"`
	TotalCost       float64 `json:"costoTotal"`
	Dimension       string  `json:"dimension"`
	Category        string  `json:"categoria"`
	Brand           string  `json:"marca"`
	Address         string  `json:"direccion"`
	Company         string  `json:"empresa"`
}
