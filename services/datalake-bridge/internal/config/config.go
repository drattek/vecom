// Package config carga la configuración de datalake-bridge desde variables de
// entorno. El servicio es autónomo (no comparte proceso ni base con el
// orchestrator/synapse-bridge): sus credenciales de Odoo y del datalake vienen
// por env, que en Azure Container Apps son secretos, igual que el resto de vecom.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config reúne todo lo que el servicio necesita para correr una pasada de sync.
type Config struct {
	// Datalake (Fabric SQL analytics endpoint, solo lectura). Se reutilizan las
	// mismas variables FABRIC_* que ya usa synapse-bridge, para no duplicar
	// secretos ni credenciales del service principal.
	FabricJDBCURL  string // FABRIC_JDBC_URL (formato JDBC de synapse-bridge; se parsea host/db)
	FabricClientID string // FABRIC_USERNAME (app id del service principal)
	FabricSecret   string // FABRIC_PASSWORD (secreto del service principal)
	AzureTenantID  string // AZURE_TENANT_ID (tenant del service principal)

	// Odoo (API externa JSON-2). Mismo Odoo de producción (refaccionesvegusa.mx).
	OdooBaseURL  string // ODOO_URL
	OdooAPIKey   string // ODOO_API_KEY
	OdooDatabase string // ODOO_DATABASE

	// Comportamiento del sync.
	Interval     time.Duration // SYNC_MACHINES_INTERVAL (0 => una sola pasada y salir)
	RunOnAtStart bool          // SYNC_MACHINES_RUN_AT_START (corre al arrancar aunque haya Interval)
	DryRun       bool          // SYNC_MACHINES_DRY_RUN (calcula el diff pero no escribe en Odoo)

	// Categoría de producto de Odoo bajo la que viven las máquinas.
	MachineCategory string // ODOO_MACHINE_CATEGORY (default "Máquinas")

	// Tenant por defecto del service principal (mismo que deploy_views.py).
	tenantFallback string
}

const defaultTenant = "e60ad600-4568-4f55-bdad-195ca7bc2861"

// Load lee la configuración del entorno y valida lo imprescindible.
func Load() (Config, error) {
	c := Config{
		FabricJDBCURL:   os.Getenv("FABRIC_JDBC_URL"),
		FabricClientID:  os.Getenv("FABRIC_USERNAME"),
		FabricSecret:    os.Getenv("FABRIC_PASSWORD"),
		AzureTenantID:   firstNonEmpty(os.Getenv("AZURE_TENANT_ID"), defaultTenant),
		OdooBaseURL:     strings.TrimRight(os.Getenv("ODOO_URL"), "/"),
		OdooAPIKey:      os.Getenv("ODOO_API_KEY"),
		OdooDatabase:    os.Getenv("ODOO_DATABASE"),
		RunOnAtStart:    boolEnv("SYNC_MACHINES_RUN_AT_START", true),
		DryRun:          boolEnv("SYNC_MACHINES_DRY_RUN", false),
		MachineCategory: firstNonEmpty(os.Getenv("ODOO_MACHINE_CATEGORY"), "Máquinas"),
	}

	if v := strings.TrimSpace(os.Getenv("SYNC_MACHINES_INTERVAL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SYNC_MACHINES_INTERVAL inválido (%q): %w", v, err)
		}
		c.Interval = d
	}

	var missing []string
	for name, val := range map[string]string{
		"FABRIC_JDBC_URL": c.FabricJDBCURL,
		"FABRIC_USERNAME": c.FabricClientID,
		"FABRIC_PASSWORD": c.FabricSecret,
		"ODOO_URL":        c.OdooBaseURL,
		"ODOO_API_KEY":    c.OdooAPIKey,
		"ODOO_DATABASE":   c.OdooDatabase,
	} {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("faltan variables de entorno: %s", strings.Join(missing, ", "))
	}

	return c, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func boolEnv(name string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
