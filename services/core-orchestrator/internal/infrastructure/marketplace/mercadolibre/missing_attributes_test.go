package mercadolibre

import (
	"errors"
	"reflect"
	"testing"
)

func TestMissingAttributeIDsFromBody(t *testing.T) {
	body := []byte(`{"message":"Validation error","cause":[
		{"code":"item.attributes.missing_required","message":"The attributes [AGE_GROUP, GENDER] are required for category MLM1 and channel marketplace."},
		{"code":"missing.fashion_grid.grid_row_id.values","message":"Attribute [SIZE_GRID_ROW_ID] is missing"},
		{"code":"item.attributes.mandatory","message":"Attribute COLOR is mandatory","attribute":"COLOR"},
		{"code":"item.price.invalid","message":"Price [ABC] invalid"}
	]}`)

	got := missingAttributeIDsFromBody(body)
	want := []string{"AGE_GROUP", "GENDER", "COLOR"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMissingAttributesErrorUnwrap(t *testing.T) {
	inner := errors.New("boom")
	err := error(&MissingAttributesError{AttributeIDs: []string{"AGE_GROUP"}, Err: inner})
	var target *MissingAttributesError
	if !errors.As(err, &target) || !errors.Is(err, inner) {
		t.Fatal("expected errors.As/Is to work through MissingAttributesError")
	}
}
