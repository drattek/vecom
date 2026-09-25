#!/usr/bin/env python3
"""Despliega las vistas dyn.* de dyn_views.sql en el lakehouse del Link to Fabric.

Usa el mismo service principal y la misma URL que synapse-bridge (FABRIC_* de
vecom.env), así que si esto funciona, la app también puede leer las vistas.

    python3 infrastructure/fabric/deploy_views.py            # crea/reemplaza las vistas
    python3 infrastructure/fabric/deploy_views.py --check    # solo valida (no crea nada)

Requisitos: pip install msal pyodbc  +  ODBC Driver 18 for SQL Server.

Mismo patrón de conexión que Dyn365/BI/fabric_sql.py: token AAD del service
principal (scope database.windows.net) inyectado en pyodbc (SQL_COPT_SS_ACCESS_TOKEN).
Nunca imprime secretos.
"""
import argparse
import re
import struct
import sys
import time
from pathlib import Path

import msal
import pyodbc

ROOT = Path(__file__).resolve().parents[2]
SQL_FILE = Path(__file__).with_name("dyn_views.sql")
TENANT_ID = "e60ad600-4568-4f55-bdad-195ca7bc2861"
SQL_COPT_SS_ACCESS_TOKEN = 1256


def load_env(path):
    """KEY=valor, admite CRLF y comillas (mismo criterio que setup-containerapps.sh)."""
    values = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        values[key] = value
    return values


def connect(env):
    url = env["FABRIC_JDBC_URL"]
    server = re.search(r"sqlserver://([^:;/]+)", url).group(1)
    database = re.search(r"database(?:Name)?=([^;]+)", url, re.I).group(1)
    app = msal.ConfidentialClientApplication(
        env["FABRIC_USERNAME"],
        authority=f"https://login.microsoftonline.com/{env.get('AZURE_TENANT_ID', TENANT_ID)}",
        client_credential=env["FABRIC_PASSWORD"],
    )
    result = app.acquire_token_for_client(scopes=["https://database.windows.net/.default"])
    if "access_token" not in result:
        sys.exit(f"No se obtuvo token AAD: {result.get('error')}")
    token = result["access_token"].encode("utf-16-le")
    conn_str = (
        "DRIVER={ODBC Driver 18 for SQL Server};"
        f"SERVER={server};DATABASE={database};Encrypt=yes;TrustServerCertificate=no"
    )
    print(f"Conectando a {server} / {database}")
    conn = pyodbc.connect(
        conn_str,
        attrs_before={SQL_COPT_SS_ACCESS_TOKEN: struct.pack("<i", len(token)) + token},
        autocommit=True,
        timeout=60,
    )
    return conn.cursor()


def parse_views():
    """Bloques '-- @view dyn.Nombre' en el orden del archivo (dependencias primero)."""
    text = SQL_FILE.read_text()
    pattern = re.compile(r"^-- @view (dyn\.\w+)\n(.*?)(?=^-- @view |\Z)", re.S | re.M)
    return [(m.group(1), m.group(2).strip()) for m in pattern.finditer(text)]


def execute(cur, sql, retries=3):
    # El endpoint del Link da errores transitorios ("underlying location does not
    # exist", "Invalid object name") mientras sincroniza metadatos: se reintenta.
    for attempt in range(1, retries + 1):
        try:
            cur.execute(sql)
            return
        except pyodbc.Error as exc:
            transient = "underlying location" in str(exc) or "Invalid object name" in str(exc)
            if not transient or attempt == retries:
                raise
            print(f"   error transitorio, reintento {attempt}/{retries - 1} en 15 s...")
            time.sleep(15)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--check", action="store_true", help="solo valida, no crea vistas")
    parser.add_argument("--env-file", default=str(ROOT / "vecom.env"))
    args = parser.parse_args()

    cur = connect(load_env(Path(args.env_file)))
    views = parse_views()

    if args.check:
        # Sin crear nada: cada vista se valida con sus dependencias expandidas
        # como subconsultas.
        bodies = dict(views)

        def expand(sql):
            for name, body in bodies.items():
                if re.search(rf"\b{re.escape(name)}\b", sql):
                    sql = re.sub(rf"\b{re.escape(name)}\b", lambda _m, b=body: f"({expand(b)})", sql)
            return sql

        ok = True
        for name, body in views:
            try:
                execute(cur, f"SELECT COUNT(*) FROM ({expand(body)}) v")
                print(f"OK  {name}: {cur.fetchone()[0]} filas")
            except pyodbc.Error as exc:
                ok = False
                print(f"ERR {name}: {str(exc)[:300]}")
        sys.exit(0 if ok else 1)

    for name, body in views:
        print(f"-> {name}")
        # DROP + CREATE en lugar de CREATE OR ALTER: funciona en cualquier
        # versión del endpoint (ver Dyn365/Dataverse/dyn_cfdi_view_fabric.sql).
        execute(cur, f"DROP VIEW IF EXISTS {name}")
        execute(cur, f"CREATE VIEW {name} AS\n{body}")
        execute(cur, f"SELECT TOP 1 * FROM {name}")
        cur.fetchall()
    print(f"Listo: {len(views)} vistas desplegadas.")


if __name__ == "__main__":
    main()
