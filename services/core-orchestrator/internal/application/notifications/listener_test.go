package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/infrastructure/telegram"
)

type fakeSettings struct {
	current mysqlInfra.NotificationSettingsDTO
	offsets []int64
}

func (f *fakeSettings) Get(context.Context) (*mysqlInfra.NotificationSettingsDTO, error) {
	copied := f.current
	return &copied, nil
}

func (f *fakeSettings) Update(_ context.Context, input mysqlInfra.UpdateNotificationSettingsInput) error {
	f.current.BotTokenEncrypted = input.BotTokenEncrypted
	f.current.BotUsername = input.BotUsername
	f.current.IsEnabled = input.IsEnabled
	f.current.TelegramUpdateOffset = input.TelegramUpdateOffset
	return nil
}

func (f *fakeSettings) SaveUpdateOffset(_ context.Context, _ string, offset int64) error {
	f.offsets = append(f.offsets, offset)
	return nil
}

type fakeLinks struct {
	linkStore
	byCode    map[string]*mysqlInfra.TelegramLinkDTO
	byChat    map[int64]*mysqlInfra.TelegramLinkDTO
	completed []mysqlInfra.CompleteTelegramLinkInput
	findErr   error
}

func (f *fakeLinks) FindByCodeHash(_ context.Context, hash string) (*mysqlInfra.TelegramLinkDTO, error) {
	return f.byCode[hash], f.findErr
}

func (f *fakeLinks) FindByChatID(_ context.Context, chatID int64) (*mysqlInfra.TelegramLinkDTO, error) {
	return f.byChat[chatID], nil
}

func (f *fakeLinks) Complete(_ context.Context, input mysqlInfra.CompleteTelegramLinkInput) (bool, error) {
	f.completed = append(f.completed, input)
	return true, nil
}

type plainCipher struct{}

func (plainCipher) Encrypt(v string) (string, error) { return "enc:" + v, nil }
func (plainCipher) Decrypt(v string) (string, error) { return strings.TrimPrefix(v, "enc:"), nil }

type fakeBot struct {
	updates       []telegram.Update
	requestOffset int64
	sent          []string
	me            telegram.User
	meErr         error
	sendErr       error
}

func (f *fakeBot) GetMe(context.Context, string) (*telegram.User, error) { return &f.me, f.meErr }

func (f *fakeBot) SendMessage(_ context.Context, _ string, _ int64, text string) error {
	f.sent = append(f.sent, text)
	return f.sendErr
}

func (f *fakeBot) SendDocument(_ context.Context, _ string, _ int64, fileName string, _ []byte, caption string) error {
	f.sent = append(f.sent, fileName+"|"+caption)
	return f.sendErr
}

func (f *fakeBot) GetUpdates(_ context.Context, _ string, offset int64, _ time.Duration) ([]telegram.Update, error) {
	f.requestOffset = offset
	return f.updates, nil
}

var testNow = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func newTestService(links *fakeLinks, bot *fakeBot) (*Service, *fakeSettings) {
	settings := &fakeSettings{current: mysqlInfra.NotificationSettingsDTO{
		BotTokenEncrypted: "enc:token", BotUsername: "vegusa_bot", IsEnabled: true,
		DailySummaryTime: "08:00:00", TelegramUpdateOffset: 40,
	}}
	return &Service{
		settings: settings, links: links, cipher: plainCipher{}, bot: bot,
		now: func() time.Time { return testNow },
	}, settings
}

func startUpdate(id, chatID int64, text string) telegram.Update {
	return telegram.Update{UpdateID: id, Message: &telegram.Message{
		Chat: telegram.Chat{ID: chatID, Type: "private"},
		From: &telegram.User{Username: "persona", FirstName: "Nombre"},
		Text: text,
	}}
}

func pendingLink(expiresAt time.Time) map[string]*mysqlInfra.TelegramLinkDTO {
	return map[string]*mysqlInfra.TelegramLinkDTO{
		hashLinkCode("codigo"): {ID: 5, UserID: 9, Username: "ana", LinkCodeExpiresAt: &expiresAt},
	}
}

func TestPollOnceCompletesLinkAndAdvancesOffset(t *testing.T) {
	links := &fakeLinks{byCode: pendingLink(testNow.Add(time.Minute))}
	bot := &fakeBot{updates: []telegram.Update{startUpdate(41, 777, "/start codigo")}}
	service, settings := newTestService(links, bot)

	if _, err := service.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}

	if bot.requestOffset != 41 {
		t.Errorf("requested offset = %d, want 41", bot.requestOffset)
	}
	if len(links.completed) != 1 || links.completed[0].ChatID != 777 || links.completed[0].BotUsername != "vegusa_bot" {
		t.Fatalf("unexpected completion: %+v", links.completed)
	}
	if len(settings.offsets) != 1 || settings.offsets[0] != 41 {
		t.Errorf("saved offsets = %v", settings.offsets)
	}
}

func TestPollOnceRejectsInvalidLinks(t *testing.T) {
	cases := map[string]*fakeLinks{
		"expired code": {byCode: pendingLink(testNow.Add(-time.Second))},
		"unknown code": {byCode: map[string]*mysqlInfra.TelegramLinkDTO{}},
		"chat of another user": {
			byCode: pendingLink(testNow.Add(time.Minute)),
			byChat: map[int64]*mysqlInfra.TelegramLinkDTO{777: {UserID: 1, Username: "otro"}},
		},
	}

	for name, links := range cases {
		t.Run(name, func(t *testing.T) {
			bot := &fakeBot{updates: []telegram.Update{startUpdate(41, 777, "/start codigo")}}
			service, _ := newTestService(links, bot)

			if _, err := service.pollOnce(context.Background()); err != nil {
				t.Fatalf("pollOnce: %v", err)
			}
			if len(links.completed) != 0 {
				t.Fatalf("link must not be completed: %+v", links.completed)
			}
			if len(bot.sent) != 1 {
				t.Errorf("expected one reply, got %v", bot.sent)
			}
		})
	}
}

func TestPollOnceIgnoresGroupMessages(t *testing.T) {
	update := startUpdate(41, -100, "/start codigo")
	update.Message.Chat.Type = "group"
	links := &fakeLinks{byCode: pendingLink(testNow.Add(time.Minute))}
	bot := &fakeBot{updates: []telegram.Update{update}}
	service, _ := newTestService(links, bot)

	if _, err := service.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if len(links.completed) != 0 || len(bot.sent) != 0 {
		t.Fatalf("group messages must be ignored")
	}
}

func TestPollOnceKeepsOffsetWhenHandlingFails(t *testing.T) {
	links := &fakeLinks{findErr: errors.New("database down")}
	bot := &fakeBot{updates: []telegram.Update{startUpdate(41, 777, "/start codigo")}}
	service, settings := newTestService(links, bot)

	if _, err := service.pollOnce(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
	if len(settings.offsets) != 0 {
		t.Errorf("offset must not advance: %v", settings.offsets)
	}
}

func TestUpdateSettingsResetsOffsetForNewBot(t *testing.T) {
	bot := &fakeBot{me: telegram.User{Username: "otro_bot"}}
	service, settings := newTestService(&fakeLinks{}, bot)

	_, err := service.UpdateSettings(context.Background(), UpdateSettingsInput{
		BotToken: "nuevo", IsEnabled: true, DailySummaryTime: "08:00",
		ErpSilenceHours: 36, ConnectionCheckMinutes: 60,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if settings.current.BotTokenEncrypted != "enc:nuevo" || settings.current.TelegramUpdateOffset != 0 {
		t.Fatalf("unexpected settings: %+v", settings.current)
	}
}

func TestUpdateSettingsRejectsBadToken(t *testing.T) {
	bot := &fakeBot{meErr: &telegram.APIError{Method: "getMe", Code: 401}}
	service, settings := newTestService(&fakeLinks{}, bot)

	_, err := service.UpdateSettings(context.Background(), UpdateSettingsInput{
		BotToken: "malo", IsEnabled: true, DailySummaryTime: "08:00",
		ErpSilenceHours: 36, ConnectionCheckMinutes: 60,
	})
	if !errors.Is(err, ErrInvalidBotToken) {
		t.Fatalf("got %v", err)
	}
	if settings.current.BotTokenEncrypted != "enc:token" {
		t.Fatalf("the stored token must not change")
	}
}

func TestAllowedDefinitionsFollowPermissions(t *testing.T) {
	admin := &domain.AuthUser{Role: domain.AdminRole}
	if got := len(AllowedDefinitions(admin)); got != len(catalog) {
		t.Errorf("admin sees %d of %d", got, len(catalog))
	}

	limited := &domain.AuthUser{Permissions: []string{"connections.view"}}
	allowed := AllowedDefinitions(limited)
	if len(allowed) != 1 || allowed[0].Type != TypeConnectionDown {
		t.Errorf("unexpected definitions: %+v", allowed)
	}

	service, _ := newTestService(&fakeLinks{}, &fakeBot{})
	err := service.SavePreferences(context.Background(), limited, map[string]bool{TypeErpSyncFailure: true})
	if !errors.Is(err, ErrUnknownType) {
		t.Errorf("saving a type without permission: got %v", err)
	}
}
