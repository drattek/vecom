// Package datalake lee las unidades de máquina (grupo MAQ de D365) desde el
// Link to Fabric, a través de la vista dyn.MachineUnits. Es solo lectura: no
// toca el datalake, solo consulta.
package datalake

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/microsoft/go-mssqldb/azuread"
)

// Unit es una unidad física de máquina (una serie) con existencia en D365.
// Corresponde 1:1 a una fila de dyn.MachineUnits.
type Unit struct {
	Company   string // dataareaid: "msb" / "vrs"
	ItemID    string // itemid del modelo (default_code en Odoo)
	Name      string // nombre del producto (ES-MX)
	Site      string // inventsiteid: sucursal (LEN/AGS/GDL/IRP/MEX/QRO/SLP)
	Serial    string // número de serie (una unidad física)
	Batch     string // lote (puede venir vacío)
	Available int    // disponible físico (normalmente 1 por serie)
}

// Model son los datos a nivel MODELO (una fila por artículo MAQ con existencia),
// para enriquecer el producto en Odoo (marca, precio, costo, unidad, peso, volumen).
// Corresponde 1:1 a una fila de dyn.MachineModels.
type Model struct {
	Company   string  // dataareaid
	ItemID    string  // itemid (default_code)
	Name      string  // nombre ES-MX
	BrandCode string  // brandcodeid_mx (nombre legible; puede venir vacío)
	MarcaCod  string  // dim. financiera Marca (código: UCA/BOB/JLG...)
	SalePrice float64 // precio de venta (inventtablemodule moduletype=2)
	Cost      float64 // costo (inventtablemodule moduletype=0)
	Unit      string  // unidad (unitid, p.ej. "pz")
	Weight    float64 // netweight
	Volume    float64 // unitvolume
}

// Reader consulta el datalake de Fabric.
type Reader struct {
	db *sql.DB
}

var (
	reHost = regexp.MustCompile(`sqlserver://([^;/]+)`)
	reDB   = regexp.MustCompile(`(?i)database(?:Name)?=([^;]+)`)
)

// Open abre una conexión al SQL analytics endpoint de Fabric autenticándose con
// el service principal (mismo patrón que synapse-bridge/deploy_views.py: token
// AAD ActiveDirectoryServicePrincipal, scope database.windows.net).
//
// jdbcURL es la FABRIC_JDBC_URL en formato JDBC; de ahí se extraen host y base.
func Open(jdbcURL, clientID, secret, tenantID string) (*Reader, error) {
	hostMatch := reHost.FindStringSubmatch(jdbcURL)
	dbMatch := reDB.FindStringSubmatch(jdbcURL)
	if hostMatch == nil || dbMatch == nil {
		return nil, fmt.Errorf("no se pudo parsear host/database de FABRIC_JDBC_URL")
	}
	host := strings.TrimSuffix(hostMatch[1], ":1433")
	database := dbMatch[1]

	// go-mssqldb, conector azuread, autenticación por service principal.
	// user id = <clientid>@<tenantid>, password = secreto del service principal.
	dsn := fmt.Sprintf(
		"server=%s;database=%s;fedauth=ActiveDirectoryServicePrincipal;user id=%s@%s;password=%s;encrypt=true;TrustServerCertificate=false;dial timeout=30",
		host, database, clientID, tenantID, secret,
	)

	db, err := sql.Open(azuread.DriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir conexión al datalake: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(3 * time.Minute)

	return &Reader{db: db}, nil
}

// Close libera la conexión.
func (r *Reader) Close() error { return r.db.Close() }

// Ping valida que la conexión y el token AAD funcionan.
func (r *Reader) Ping(ctx context.Context) error { return r.db.PingContext(ctx) }

// Machines devuelve todas las unidades de máquina con existencia (una por serie).
func (r *Reader) Machines(ctx context.Context) ([]Unit, error) {
	const q = `SELECT [Empresa],[Articulo],[Nombre],[Sucursal],[Serie],[Lote],[Disponible]
	           FROM dyn.MachineUnits`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("consultar dyn.MachineUnits: %w", err)
	}
	defer rows.Close()

	var out []Unit
	for rows.Next() {
		var u Unit
		var name, batch sql.NullString
		if err := rows.Scan(&u.Company, &u.ItemID, &name, &u.Site, &u.Serial, &batch, &u.Available); err != nil {
			return nil, fmt.Errorf("leer fila de dyn.MachineUnits: %w", err)
		}
		u.Company = strings.ToLower(strings.TrimSpace(u.Company))
		u.Name = strings.TrimSpace(name.String)
		u.Batch = strings.TrimSpace(batch.String)
		u.Serial = strings.TrimSpace(u.Serial)
		u.Site = strings.TrimSpace(u.Site)
		u.ItemID = strings.TrimSpace(u.ItemID)
		if u.Serial == "" || u.ItemID == "" {
			continue
		}
		if u.Name == "" {
			u.Name = u.ItemID
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar dyn.MachineUnits: %w", err)
	}
	return out, nil
}

// Models devuelve los datos a nivel modelo (uno por artículo MAQ con existencia).
func (r *Reader) Models(ctx context.Context) ([]Model, error) {
	const q = `SELECT [Empresa],[Articulo],[Nombre],[BrandCode],[MarcaCod],
	                  [PrecioVenta],[Costo],[Unidad],[Peso],[Volumen]
	           FROM dyn.MachineModels`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("consultar dyn.MachineModels: %w", err)
	}
	defer rows.Close()

	var out []Model
	for rows.Next() {
		var m Model
		var name, brand, marca, unit sql.NullString
		var price, cost, weight, vol sql.NullFloat64
		if err := rows.Scan(&m.Company, &m.ItemID, &name, &brand, &marca, &price, &cost, &unit, &weight, &vol); err != nil {
			return nil, fmt.Errorf("leer fila de dyn.MachineModels: %w", err)
		}
		m.Company = strings.ToLower(strings.TrimSpace(m.Company))
		m.ItemID = strings.TrimSpace(m.ItemID)
		m.Name = strings.TrimSpace(name.String)
		m.BrandCode = strings.TrimSpace(brand.String)
		m.MarcaCod = strings.TrimSpace(marca.String)
		m.Unit = strings.TrimSpace(unit.String)
		m.SalePrice = price.Float64
		m.Cost = cost.Float64
		m.Weight = weight.Float64
		m.Volume = vol.Float64
		if m.ItemID == "" {
			continue
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar dyn.MachineModels: %w", err)
	}
	return out, nil
}
