package domain

// ListingLiveStatus compara una publicación local con su estado actual en el
// marketplace.
type ListingLiveStatus struct {
	MapID            int64
	ProductID        int64
	VehicleFitmentID *int64
	ExternalID       string
	LocalStatus      string
	// Found: el marketplace devolvió la publicación.
	Found     bool
	RawStatus string
	SubStatus []string
	// Closed: ese estado equivale a 'closed' en el mapa local.
	Closed bool
}

// ClosureReason es el motivo que da el marketplace de una moderación.
type ClosureReason struct {
	ModerationName string
	Reason         string
	Remedy         string
}
