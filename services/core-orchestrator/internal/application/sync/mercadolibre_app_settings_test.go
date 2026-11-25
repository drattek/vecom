package sync

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func TestValidateAppUpdate(t *testing.T) {
	current := MercadoLibreAppSettings{AppID: "1111111111", HasClientSecret: true}
	const redirect = "https://dashboard.ejemplo.test/meli_callback"

	cases := []struct {
		name        string
		input       UpdateMercadoLibreAppInput
		wantChanged bool
		wantError   bool
	}{
		{"misma app sin capturar secreto", UpdateMercadoLibreAppInput{AppID: " 1111111111 ", RedirectURL: redirect}, false, false},
		{"app nueva con secreto", UpdateMercadoLibreAppInput{AppID: "2222222222", ClientSecret: "abc123", RedirectURL: redirect}, true, false},
		{"app nueva sin secreto", UpdateMercadoLibreAppInput{AppID: "2222222222", RedirectURL: redirect}, false, true},
		{"app_id no numérico", UpdateMercadoLibreAppInput{AppID: "mi-app", ClientSecret: "abc", RedirectURL: redirect}, false, true},
		{"redirect sin https", UpdateMercadoLibreAppInput{AppID: "1111111111", RedirectURL: "http://dashboard.ejemplo.test/meli_callback"}, false, true},
		{"redirect con parámetros", UpdateMercadoLibreAppInput{AppID: "1111111111", RedirectURL: redirect + "?connectionId=2"}, false, true},
		{"redirect a otra ruta", UpdateMercadoLibreAppInput{AppID: "1111111111", RedirectURL: "https://dashboard.ejemplo.test/otra"}, false, true},
		{"secreto con espacios", UpdateMercadoLibreAppInput{AppID: "2222222222", ClientSecret: "abc 123", RedirectURL: redirect}, false, true},
	}
	for _, tc := range cases {
		_, changed, err := validateAppUpdate(tc.input, current)
		if tc.wantError {
			if !errors.Is(err, ErrInvalidMercadoLibreApp) {
				t.Errorf("%s: err = %v", tc.name, err)
			} else if tc.input.ClientSecret != "" && strings.Contains(err.Error(), tc.input.ClientSecret) {
				t.Errorf("%s: el error expone el secreto", tc.name)
			}
			continue
		}
		if err != nil || changed != tc.wantChanged {
			t.Errorf("%s: changed=%v err=%v", tc.name, changed, err)
		}
	}

	// Sin secreto guardado hay que capturarlo aunque la app no cambie.
	if _, _, err := validateAppUpdate(UpdateMercadoLibreAppInput{AppID: "1111111111", RedirectURL: redirect}, MercadoLibreAppSettings{AppID: "1111111111"}); err == nil {
		t.Error("debe pedir el client_secret")
	}
}

func TestDescribeAppSettingsHidesSecrets(t *testing.T) {
	described := describeAppSettings(mercadoLibreSettingsByKey([]mysqlInfra.ConnectionSettingDTO{
		{KeyName: "APP_ID", Value: "1111111111"},
		{KeyName: "client_secret", Value: "super-secreto"},
		{KeyName: "access_token", Value: "token-secreto"},
		{KeyName: "meli_user_id", Value: "987"},
	}))
	payload, err := json.Marshal(described)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secreto") {
		t.Errorf("la respuesta expone secretos: %s", payload)
	}
	if !described.HasClientSecret || described.LinkedAccountID == nil || *described.LinkedAccountID != 987 || described.AppID != "1111111111" {
		t.Errorf("descripción = %+v", described)
	}
}
