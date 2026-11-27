// Package telegram es el transporte de la Bot API de Telegram (ADR 0012). No
// tiene lógica de negocio.
//
// El token del bot viaja en la URL: ningún error que salga de este paquete lo
// contiene.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.telegram.org"
	requestTimeout = 15 * time.Second
	uploadTimeout  = 60 * time.Second
	// maxResponseBytes acota la respuesta que se lee de Telegram.
	maxResponseBytes = 4 << 20
)

// ErrMalformedToken indica que el texto no tiene forma de token de bot.
// Validarlo evita además que altere la ruta de la URL.
var ErrMalformedToken = errors.New("malformed telegram bot token")

var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]{20,}$`)

// APIError es una respuesta con ok=false de la Bot API.
type APIError struct {
	Method      string
	Code        int
	Description string
	// RetryAfter es la espera que pide Telegram al exceder sus límites.
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %s: %d %s", e.Method, e.Code, e.Description)
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	From *User  `json:"from"`
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

// ChatMemberUpdated avisa, en un chat privado, que el usuario bloqueó
// (status "kicked") o desbloqueó (status "member") al bot.
type ChatMemberUpdated struct {
	Chat          Chat `json:"chat"`
	NewChatMember struct {
		Status string `json:"status"`
	} `json:"new_chat_member"`
}

type Update struct {
	UpdateID     int64              `json:"update_id"`
	Message      *Message           `json:"message"`
	MyChatMember *ChatMemberUpdated `json:"my_chat_member"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient crea el cliente. Con httpClient nil usa uno propio; el plazo de
// cada petición se fija por contexto porque getUpdates espera más que el resto.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{httpClient: httpClient, baseURL: defaultBaseURL}
}

// GetMe devuelve los datos del bot; sirve para validar el token.
func (c *Client) GetMe(ctx context.Context, token string) (*User, error) {
	var me User
	if err := c.call(ctx, token, "getMe", requestTimeout, map[string]any{}, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// SendMessage envía texto plano (sin parse_mode, para no tener que escapar).
func (c *Client) SendMessage(ctx context.Context, token string, chatID int64, text string) error {
	return c.call(ctx, token, "sendMessage", requestTimeout, map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, nil)
}

// SendDocument envía un archivo con un texto de hasta 1024 caracteres.
func (c *Client) SendDocument(ctx context.Context, token string, chatID int64, fileName string, content []byte, caption string) error {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("telegram sendDocument: encoding request: %w", err)
	}
	if caption != "" {
		if err := form.WriteField("caption", caption); err != nil {
			return fmt.Errorf("telegram sendDocument: encoding request: %w", err)
		}
	}
	file, err := form.CreateFormFile("document", fileName)
	if err != nil {
		return fmt.Errorf("telegram sendDocument: encoding request: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("telegram sendDocument: encoding request: %w", err)
	}
	if err := form.Close(); err != nil {
		return fmt.Errorf("telegram sendDocument: encoding request: %w", err)
	}
	return c.do(ctx, token, "sendDocument", uploadTimeout, form.FormDataContentType(), body.Bytes(), nil)
}

// GetUpdates hace long polling: espera hasta wait a que haya algo nuevo.
// offset es el primer update_id que se quiere recibir.
func (c *Client) GetUpdates(ctx context.Context, token string, offset int64, wait time.Duration) ([]Update, error) {
	var updates []Update
	if err := c.call(ctx, token, "getUpdates", wait+requestTimeout, map[string]any{
		"offset":          offset,
		"timeout":         int(wait.Seconds()),
		"allowed_updates": []string{"message", "my_chat_member"},
	}, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) call(ctx context.Context, token, method string, timeout time.Duration, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram %s: encoding request: %w", method, err)
	}
	return c.do(ctx, token, method, timeout, "application/json", body, result)
}

func (c *Client) do(ctx context.Context, token, method string, timeout time.Duration, contentType string, body []byte, result any) error {
	if !tokenPattern.MatchString(token) {
		return ErrMalformedToken
	}

	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.baseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return transportError(method, token, err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("telegram %s: %w", method, ctxErr)
		}
		return transportError(method, token, err)
	}
	defer resp.Body.Close()

	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram %s: invalid response (HTTP %d)", method, resp.StatusCode)
	}

	if !envelope.OK {
		code := envelope.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &APIError{
			Method:      method,
			Code:        code,
			Description: redact(envelope.Description, token),
			RetryAfter:  time.Duration(envelope.Parameters.RetryAfter) * time.Second,
		}
	}

	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("telegram %s: decoding result: %w", method, err)
	}
	return nil
}

// transportError descarta la URL que net/http agrega a sus errores. No usa %w:
// el error original la conserva.
func transportError(method, token string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return fmt.Errorf("telegram %s: %s", method, redact(err.Error(), token))
}

func redact(text, token string) string {
	return strings.ReplaceAll(text, token, "<token>")
}
