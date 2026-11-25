package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	syncApp "core-orchestrator/internal/application/sync"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type fakeOAuthFlow struct {
	result    syncApp.OAuthResult
	err       error
	completed int
	cancelled int
}

func (f *fakeOAuthFlow) Start(context.Context, int64, int64, bool) (string, error) { return "", nil }

func (f *fakeOAuthFlow) Complete(context.Context, string, string) (syncApp.OAuthResult, error) {
	f.completed++
	return f.result, f.err
}

func (f *fakeOAuthFlow) Cancel(context.Context, string) (syncApp.OAuthResult, error) {
	f.cancelled++
	return f.result, nil
}

func (f *fakeOAuthFlow) AppSettings(context.Context, int64) (*syncApp.MercadoLibreAppSettings, error) {
	return &syncApp.MercadoLibreAppSettings{}, nil
}

func (f *fakeOAuthFlow) UpdateAppSettings(context.Context, int64, int64, syncApp.UpdateMercadoLibreAppInput) (*syncApp.UpdateMercadoLibreAppResult, error) {
	return &syncApp.UpdateMercadoLibreAppResult{}, nil
}

func (f *fakeOAuthFlow) Backups(context.Context, int64) ([]mysqlInfra.ConnectionBackupDTO, error) {
	return nil, nil
}

func (f *fakeOAuthFlow) Restore(context.Context, int64, int64, int64) (*syncApp.MercadoLibreRestoreResult, error) {
	return &syncApp.MercadoLibreRestoreResult{}, nil
}

func callback(t *testing.T, flow *fakeOAuthFlow, query string) (*httptest.ResponseRecorder, url.Values) {
	t.Helper()
	recorder := httptest.NewRecorder()
	NewMercadoLibreOAuthHandler(flow).Callback(recorder, httptest.NewRequest(http.MethodGet, "/meli_callback?"+query, nil))

	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil || recorder.Code != http.StatusSeeOther || location.Path != connectionsPagePath || location.Host != "" {
		t.Fatalf("status %d, location %q (%v)", recorder.Code, recorder.Header().Get("Location"), err)
	}
	return recorder, location.Query()
}

func TestOAuthCallbackRedirectsOnSuccess(t *testing.T) {
	flow := &fakeOAuthFlow{result: syncApp.OAuthResult{ConnectionID: 7, Account: "VENDEDOR MAQ"}}
	recorder, query := callback(t, flow, "code=secreto&state=abc")

	if query.Get("meliAuth") != "success" || query.Get("connectionId") != "7" || query.Get("account") != "VENDEDOR MAQ" {
		t.Errorf("query = %v", query)
	}
	if recorder.Header().Get("Referrer-Policy") != "no-referrer" || strings.Contains(recorder.Header().Get("Location"), "secreto") {
		t.Error("el code no debe propagarse")
	}
}

func TestOAuthCallbackReportsReasonWithoutErrorText(t *testing.T) {
	flow := &fakeOAuthFlow{
		result: syncApp.OAuthResult{ConnectionID: 7, Account: "OTRA"},
		err:    fmt.Errorf("detalle interno: %w", syncApp.ErrOAuthAccountMismatch),
	}
	recorder, query := callback(t, flow, "code=x&state=abc")

	if query.Get("meliAuth") != "error" || query.Get("reason") != "account_mismatch" || query.Has("account") {
		t.Errorf("query = %v", query)
	}
	if strings.Contains(recorder.Header().Get("Location"), "detalle") {
		t.Error("el texto del error no debe llegar a la URL")
	}
}

func TestOAuthCallbackWhenUserDeniesDoesNotExchange(t *testing.T) {
	flow := &fakeOAuthFlow{result: syncApp.OAuthResult{ConnectionID: 7}}
	_, query := callback(t, flow, "error=access_denied&state=abc")

	if flow.completed != 0 || flow.cancelled != 1 || query.Get("reason") != "denied" {
		t.Errorf("completed=%d cancelled=%d query=%v", flow.completed, flow.cancelled, query)
	}
}
