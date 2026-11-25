package sync

// Prueba contra un MySQL real DESECHABLE (se omite sin la variable); simula
// MercadoLibre. Requiere una base cargada con eco-architecture.sql y
// 2026-11-25_connection_settings_backups.sql; inserta datos y renombra
// temporalmente la tabla de respaldos. Nunca apuntarla a una base real.
//
//	BACKUPS_TEST_MYSQL_DSN='root:pw@tcp(127.0.0.1:33307)/ecommerce?parseTime=true&loc=Local' \
//	  go test ./internal/application/sync/ -run Integration -v

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func TestIntegrationMercadoLibreBackups(t *testing.T) {
	dsn := os.Getenv("BACKUPS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BACKUPS_TEST_MYSQL_DSN no está definido")
	}
	// Clave ficticia de 32 bytes. Va en base64 porque el cargador intenta
	// primero esa forma.
	t.Setenv("CONNECTION_CREDENTIALS_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	ctx := context.Background()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	exec := func(query string, args ...any) int64 {
		t.Helper()
		result, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}

	// MercadoLibre simulado: la cuenta 777 con cualquier token. Como el real,
	// solo canjea el code si el verificador corresponde a la huella enviada.
	var challenge string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/me":
			_, _ = w.Write([]byte(`{"id":777,"nickname":"VENDEDOR"}`))
		case "/oauth/token":
			if r.FormValue("grant_type") == "authorization_code" && pkceChallenge(r.FormValue("code_verifier")) != challenge {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"acceso-nuevo","refresh_token":"refresco-nuevo","expires_in":21600,"user_id":777}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := mercadoLibreInfra.NewClient(server.Client(), server.URL, nil)

	suffix := time.Now().Format("150405.000000")
	user := exec("INSERT INTO ecom_api_user (username, password_hash) VALUES (?, 'x')", "bk"+suffix)
	currency := exec("INSERT INTO ecom_currencies (name, code, symbol, created_by) VALUES ('Peso', ?, '$', ?)", "B"+suffix, user)
	var channel int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM ecom_channels WHERE code = 'MERCADOLIBRE'").Scan(&channel); err != nil {
		channel = exec("INSERT INTO ecom_channels (name, code, created_by) VALUES ('MercadoLibre', 'MERCADOLIBRE', ?)", user)
	}
	connection := exec("INSERT INTO ecom_channel_connections (channel_id, name, status, currency_id, created_by) VALUES (?, 'ML respaldo', 'active', ?, ?)", channel, currency, user)

	settingsRepository := mysqlInfra.NewConnectionSettingsRepository(db)
	expiration := time.Now().UTC().Add(5 * time.Hour).Format(time.RFC3339)
	for key, value := range map[string]string{
		"app_id": "1111111111", "client_secret": "secreto-viejo", "redirect_url": "https://viejo.test/meli_callback",
		"access_token": "acceso-viejo", "refresh_token": "refresco-viejo", "expiration_time": expiration, "site_id": "MLM",
	} {
		if _, err := settingsRepository.Create(ctx, mysqlInfra.CreateConnectionSettingInput{
			ConnectionID: connection, KeyName: key, Value: value, IsEncrypted: key != "app_id" && key != "site_id", CreatedBy: user,
		}); err != nil {
			t.Fatal(err)
		}
	}
	current := func() map[string]mysqlInfra.ConnectionSettingDTO {
		t.Helper()
		settings, err := settingsRepository.FindByConnectionID(ctx, connection)
		if err != nil {
			t.Fatal(err)
		}
		return mercadoLibreSettingsByKey(settings)
	}

	tokens := NewMercadoLibreTokenService(settingsRepository, mysqlInfra.NewConnectionStatusRepository(db), nil)
	tokens.authHandler = mercadoLibreInfra.NewAuthHandler(client)
	service := NewMercadoLibreOAuthService(db, tokens, mysqlInfra.NewChannelConnectionRepository(db), mercadoLibreInfra.NewUsersHandler(client), memoryStates{})
	newApp := UpdateMercadoLibreAppInput{AppID: "2222222222", ClientSecret: "secreto-nuevo", RedirectURL: "https://nuevo.test/meli_callback"}

	// 1) Si el respaldo falla, no cambia nada.
	exec("RENAME TABLE ecom_connection_settings_backups TO ecom_connection_settings_backups_off")
	_, err = service.UpdateAppSettings(ctx, connection, user, newApp)
	exec("RENAME TABLE ecom_connection_settings_backups_off TO ecom_connection_settings_backups")
	if err == nil || current()["app_id"].Value != "1111111111" || current()["client_secret"].Value != "secreto-viejo" {
		t.Fatalf("sin respaldo no debe cambiar nada: %v", err)
	}

	// 2) Editar la aplicación respalda lo anterior y registra la cuenta.
	updated, err := service.UpdateAppSettings(ctx, connection, user, newApp)
	if err != nil {
		t.Fatalf("UpdateAppSettings: %v", err)
	}
	if !updated.AppChanged || updated.LinkedAccountID == nil || *updated.LinkedAccountID != 777 {
		t.Fatalf("resultado = %+v", updated)
	}
	if current()["client_secret"].Value != "secreto-nuevo" || !current()["client_secret"].IsEncrypted || current()["app_id"].IsEncrypted {
		t.Fatal("ajustes de la app mal guardados")
	}
	backups, err := service.Backups(ctx, connection)
	if err != nil || len(backups) != 1 || backups[0].Reason != backupReasonAppUpdate || *backups[0].AppID != "1111111111" || backups[0].CreatedBy == nil {
		t.Fatalf("respaldos = %+v (%v)", backups, err)
	}
	original := backups[0].ID
	var payload string
	if err := db.QueryRowContext(ctx, "SELECT payload FROM ecom_connection_settings_backups WHERE id = ?", original).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "secreto") || strings.Contains(payload, "acceso") || strings.Contains(payload, "client_secret") {
		t.Fatal("el respaldo no está cifrado")
	}

	// 3) Vincular también respalda.
	authorizationURL, err := service.Start(ctx, connection, user, false)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	parsed, _ := url.Parse(authorizationURL)
	challenge = parsed.Query().Get("code_challenge")
	if _, err := service.Complete(ctx, parsed.Query().Get("state"), "codigo"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if current()["access_token"].Value != "acceso-nuevo" {
		t.Fatal("no se guardaron los tokens nuevos")
	}

	// 4) Restaurar repone todo junto y quita lo que no estaba.
	restored, err := service.Restore(ctx, connection, original, user)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	settings := current()
	if settings["app_id"].Value != "1111111111" || settings["client_secret"].Value != "secreto-viejo" ||
		settings["access_token"].Value != "acceso-viejo" || settings["refresh_token"].Value != "refresco-viejo" {
		t.Fatalf("restauración incompleta: app %s", settings["app_id"].Value)
	}
	if _, found := settings["meli_user_id"]; found || restored.LinkedAccountID != nil {
		t.Error("la cuenta registrada después del respaldo debe quitarse")
	}
	if settings["site_id"].Value != "MLM" || !restored.AccessVerified || settings["app_id"].IsEncrypted || !settings["client_secret"].IsEncrypted {
		t.Errorf("restauración: verificada=%v", restored.AccessVerified)
	}
	backups, _ = service.Backups(ctx, connection)
	if len(backups) != 3 || backups[0].Reason != backupReasonRestore {
		t.Fatalf("respaldos tras restaurar = %+v", backups)
	}

	// 5) Solo quedan visibles 5, y se puede restaurar el más antiguo de ellos.
	for i := 0; i < 4; i++ {
		if _, err := service.UpdateAppSettings(ctx, connection, user, UpdateMercadoLibreAppInput{AppID: "1111111111", RedirectURL: "https://viejo.test/meli_callback"}); err != nil {
			t.Fatalf("UpdateAppSettings %d: %v", i, err)
		}
	}
	backups, _ = service.Backups(ctx, connection)
	var total int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_connection_settings_backups WHERE connection_id = ?", connection).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if len(backups) != visibleBackupsPerConnection || total != 7 {
		t.Fatalf("visibles = %d, en total = %d", len(backups), total)
	}
	if _, err := service.Restore(ctx, connection, backups[len(backups)-1].ID, user); err != nil {
		t.Errorf("restaurar el más antiguo: %v", err)
	}
	if _, err := service.Restore(ctx, connection, original, user); !errors.Is(err, mysqlInfra.ErrConnectionBackupNotFound) {
		t.Errorf("un respaldo oculto no se puede restaurar: %v", err)
	}

	// 6) Un respaldo solo se restaura en su conexión.
	other := exec("INSERT INTO ecom_channel_connections (channel_id, name, status, currency_id, created_by) VALUES (?, 'ML otra', 'active', ?, ?)", channel, currency, user)
	backups, _ = service.Backups(ctx, connection)
	if _, err := service.Restore(ctx, other, backups[0].ID, user); !errors.Is(err, mysqlInfra.ErrConnectionBackupNotFound) {
		t.Errorf("restauración cruzada: %v", err)
	}
}
