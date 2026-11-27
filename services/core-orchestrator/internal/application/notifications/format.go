package notifications

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

const (
	// maxMessageRunes deja margen bajo el límite de 4096 de Telegram.
	maxMessageRunes = 3900
	// maxCaptionRunes es el límite del texto que acompaña a un archivo.
	maxCaptionRunes = 1000
	// maxErrorRunes recorta cada error dentro de un mensaje.
	maxErrorRunes = 300
)

var secretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// La respuesta cruda de un marketplace puede traer datos de clientes.
	{regexp.MustCompile(`(?s)\s*—?\s*raw response:.*$`), ""},
	// Parámetros y campos con credenciales.
	{regexp.MustCompile(`(?i)("?(?:access_token|refresh_token|client_secret|api_?key|password|passwd|pwd|secret|authorization|code_verifier)"?\s*[=:]\s*"?)(?:bearer\s+)?[^\s"&,}]+`), "${1}<oculto>"},
	{regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`), "Bearer <oculto>"},
	// Tokens de MercadoLibre.
	{regexp.MustCompile(`\b(?:APP_USR|TG)-[A-Za-z0-9\-]+`), "<oculto>"},
	// usuario:contraseña en URLs y cadenas de conexión.
	{regexp.MustCompile(`://[^/\s@]+:[^/\s@]+@`), "://<oculto>@"},
	{regexp.MustCompile(`[^\s:@"]+:[^\s@"]+@tcp\(`), "<oculto>@tcp("},
	// Correos (datos de clientes).
	{regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`), "<correo>"},
}

// sanitize quita credenciales y datos de clientes: los mensajes salen a un
// servicio externo (ADR 0012).
func sanitize(text string) string {
	for _, secret := range secretPatterns {
		text = secret.pattern.ReplaceAllString(text, secret.replacement)
	}
	return strings.TrimSpace(text)
}

func truncate(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-1]) + "…"
}

// shortError es un error listo para ir dentro de un mensaje.
func shortError(text string) string {
	return truncate(sanitize(text), maxErrorRunes)
}

// signatureOf identifica un conjunto de valores sin importar su orden.
func signatureOf(values []string) string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// buildCSV arma un CSV que Excel abre bien: con BOM y sin celdas que se
// interpreten como fórmula.
func buildCSV(header []string, rows [][]string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	writer.UseCRLF = true
	// Escribir en memoria no falla.
	_ = writer.Write(header)
	for _, row := range rows {
		safeRow := make([]string, len(row))
		for i, cell := range row {
			if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
				cell = "'" + cell
			}
			safeRow[i] = cell
		}
		_ = writer.Write(safeRow)
	}
	writer.Flush()
	return buffer.Bytes()
}
