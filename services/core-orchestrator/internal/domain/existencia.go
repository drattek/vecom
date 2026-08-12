package domain

type Existencia struct {
	Code           string  `json:"code"`
	Type           string  `json:"type"`
	Stock          int     `json:"stock"`
	CostProm       float64 `json:"costProm"`
	Status         string  `json:"status"`
	Classification string  `json:"classification"`
	Description    string  `json:"description"`
	Unit           string  `json:"unit"`
	AgencyName     string  `json:"agencyName"`
	Original       bool    `json:"original"`
	Price          float64 `json:"price"`
	Price2         float64 `json:"price2"`
	Price3         float64 `json:"price3"`
	Price4         float64 `json:"price4"`
	Price5         float64 `json:"price5"`
	// SupersededByPartNumber es PROD_SUPERSESION en el ERP Nissan: el tag json debe
	// coincidir exactamente con el nombre de componente del record Java
	// NissanExistenciasDTO (synapse-bridge/dto/NissanExistenciasDTO.java), igual que en
	// events.go, porque Jackson serializa por nombre de componente sin transformaciones.
	SupersededByPartNumber string `json:"supersededByPartNumber"`
}
