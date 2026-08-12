package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	attributesApp "core-orchestrator/internal/application/attributes"
	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ChannelAttributeValueHandler — write side of setting a MercadoLibre custom
// attribute value on a product by sku + external_key, resolving/creating the
// ecom_attributes/ecom_channel_attributes/ecom_channel_attribute_map chain
// behind it (see channel_attribute_values.Service.SetValue). MercadoLibre-only
// for now, mirroring the underlying service.

// setChannelAttributeValueRequest.Value is decoded generically and typed
// against DataType by parseChannelAttributeValue: when DataType is omitted,
// it's inferred from Value's own JSON shape (string/number/boolean), which
// covers the common case without the caller needing to know the target
// attribute's data_type up front.
type setChannelAttributeValueRequest struct {
	SKU         string          `json:"sku"`
	ExternalKey string          `json:"externalKey"`
	Value       json.RawMessage `json:"value"`
	DataType    string          `json:"dataType,omitempty"`
}

type ChannelAttributeValueHandler struct {
	service *channelAttributeValuesApp.Service
}

func NewChannelAttributeValueHandler(service *channelAttributeValuesApp.Service) *ChannelAttributeValueHandler {
	return &ChannelAttributeValueHandler{service: service}
}

func (h *ChannelAttributeValueHandler) SetMercadoLibreValue(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req setChannelAttributeValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	parsed, err := parseChannelAttributeValue(req.Value, req.DataType)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.SetValue(r.Context(), channelAttributeValuesApp.SetValueInput{
		SKU:         req.SKU,
		ExternalKey: req.ExternalKey,
		DataType:    parsed.dataType,
		ValueText:   parsed.valueText,
		ValueNumber: parsed.valueNumber,
		ValueBool:   parsed.valueBool,
		ValueDate:   parsed.valueDate,
		EnumValue:   parsed.enumValue,
		ActorID:     user.ID,
	})
	if err != nil {
		if errors.Is(err, channelAttributeValuesApp.ErrInvalidSetValueInput) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			writeJSONError(w, http.StatusNotFound, "product not found for sku")
			return
		}
		if errors.Is(err, mysqlInfra.ErrChannelNotFound) {
			writeJSONError(w, http.StatusInternalServerError, "mercadolibre channel is not configured (no ecom_channels row with code MERCADOLIBRE)")
			return
		}
		if errors.Is(err, channelAttributeValuesApp.ErrAttributeDataTypeConflict) || errors.Is(err, channelAttributeValuesApp.ErrChannelAttributeMapConflict) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, attributesApp.ErrProductAttributeValueMismatch) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// provisionCategoryAttributesRequest is the body for
// ChannelAttributeValueHandler.ProvisionMercadoLibreCategoryAttributes:
// CategoryID is MercadoLibre's own category id (e.g. "MLM3728"), not a local
// ecom_categories one. ConnectionID is optional — only needed so a product
// with no local category yet can have one built and assigned from
// MercadoLibre's own category tree (see
// channel_attribute_values.Service.ProvisionCategoryAttributes); without it,
// such a product just gets channel-wide attribute slots instead.
type provisionCategoryAttributesRequest struct {
	SKU          string `json:"sku"`
	CategoryID   string `json:"categoryId"`
	ConnectionID *int64 `json:"connectionId,omitempty"`
}

// ProvisionMercadoLibreCategoryAttributes seeds the ecom_channel_attributes/
// ecom_channel_attribute_map rows every attribute MercadoLibre requires for
// input.CategoryID needs, given only a product's sku and that category id —
// see channel_attribute_values.Service.ProvisionCategoryAttributes for the
// system_field-vs-custom_attribute split (dimensions/brand/part_number/sku
// come from elsewhere; everything else becomes a managed custom attribute).
// It never writes a product_attributes value itself — use
// SetMercadoLibreValue per attribute for that once the slots exist.
func (h *ChannelAttributeValueHandler) ProvisionMercadoLibreCategoryAttributes(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req provisionCategoryAttributesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ProvisionCategoryAttributes(r.Context(), channelAttributeValuesApp.ProvisionCategoryAttributesInput{
		SKU:          req.SKU,
		CategoryID:   req.CategoryID,
		ConnectionID: req.ConnectionID,
		ActorID:      user.ID,
	})
	if err != nil {
		if errors.Is(err, channelAttributeValuesApp.ErrInvalidProvisionInput) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			writeJSONError(w, http.StatusNotFound, "product not found for sku")
			return
		}
		if errors.Is(err, mysqlInfra.ErrChannelNotFound) {
			writeJSONError(w, http.StatusInternalServerError, "mercadolibre channel is not configured (no ecom_channels row with code MERCADOLIBRE)")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// parsedChannelAttributeValue is the outcome of interpreting a request's raw
// JSON "value" against its (explicit or inferred) dataType — exactly one of
// the value fields is set, matching dataType.
type parsedChannelAttributeValue struct {
	dataType    string
	valueText   *string
	valueNumber *float64
	valueBool   *bool
	valueDate   *time.Time
	enumValue   *string
}

// parseChannelAttributeValue types raw against declaredType, or — when
// declaredType is empty — infers it from raw's own JSON shape: a JSON string
// becomes "text", a JSON number becomes "number", a JSON boolean becomes
// "boolean". "date" and "enum" can't be told apart from a plain JSON string,
// so both require declaredType to be set explicitly.
func parseChannelAttributeValue(raw json.RawMessage, declaredType string) (parsedChannelAttributeValue, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return parsedChannelAttributeValue{}, errors.New("value is required")
	}

	dataType := strings.TrimSpace(declaredType)
	if dataType == "" {
		switch value.(type) {
		case string:
			dataType = "text"
		case float64:
			dataType = "number"
		case bool:
			dataType = "boolean"
		default:
			return parsedChannelAttributeValue{}, errors.New("value must be a string, number or boolean when dataType is not specified")
		}
	}

	switch dataType {
	case "text":
		text, ok := value.(string)
		if !ok {
			return parsedChannelAttributeValue{}, errors.New("value must be a string for dataType text")
		}
		return parsedChannelAttributeValue{dataType: dataType, valueText: &text}, nil

	case "number":
		switch v := value.(type) {
		case float64:
			return parsedChannelAttributeValue{dataType: dataType, valueNumber: &v}, nil
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return parsedChannelAttributeValue{}, errors.New("value must be a number for dataType number")
			}
			return parsedChannelAttributeValue{dataType: dataType, valueNumber: &parsed}, nil
		default:
			return parsedChannelAttributeValue{}, errors.New("value must be a number for dataType number")
		}

	case "boolean":
		boolValue, ok := value.(bool)
		if !ok {
			return parsedChannelAttributeValue{}, errors.New("value must be a boolean for dataType boolean")
		}
		return parsedChannelAttributeValue{dataType: dataType, valueBool: &boolValue}, nil

	case "date":
		text, ok := value.(string)
		if !ok {
			return parsedChannelAttributeValue{}, errors.New("value must be a \"YYYY-MM-DD\" string for dataType date")
		}
		parsed, err := time.Parse("2006-01-02", text)
		if err != nil {
			return parsedChannelAttributeValue{}, errors.New("value must be a \"YYYY-MM-DD\" string for dataType date")
		}
		return parsedChannelAttributeValue{dataType: dataType, valueDate: &parsed}, nil

	case "enum":
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return parsedChannelAttributeValue{}, errors.New("value must be a non-empty string for dataType enum")
		}
		return parsedChannelAttributeValue{dataType: dataType, enumValue: &text}, nil

	default:
		return parsedChannelAttributeValue{}, errors.New("dataType must be one of text, number, boolean, date, enum")
	}
}
