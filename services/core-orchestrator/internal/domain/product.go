package domain

type Product struct {
	Id          int    `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	PartNumber  string `json:"partNumber"`
	Group       string `json:"group"`
	Brand       string `json:"brand"`
}
