package domain

// Los tags json deben coincidir exactamente con los nombres de componente de los
// records Java en synapse-bridge (dto/SyncCompletedEvent.java, SyncFailedEvent.java,
// PageProcessedEvent.java), ya que Jackson2JsonMessageConverter serializa por nombre
// de componente sin transformaciones (p.ej. "totalRecord", no "totalRecords").
type SyncCompletedEvent struct {
	Source       string `json:"source"`
	TotalRecords int    `json:"totalRecord"`
	Pages        int    `json:"pages"`
}

type SyncFailedEvent struct {
	Source    string `json:"source"`
	Offset    int    `json:"offset"`
	PageSize  int    `json:"pageSize"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

type PageProcessedEvent struct {
	Source  string `json:"source"`
	Page    int    `json:"page"`
	Offset  int    `json:"offset"`
	Records int    `json:"records"`
}
