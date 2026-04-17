package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"vpn-backend/internal/infra/backendapi"
	botqrcode "vpn-backend/internal/infra/qrcode"
	"vpn-backend/internal/infra/telegram"
)

type Bot struct {
	logger            *slog.Logger
	telegram          telegramClient
	backend           backendClient
	pollTimeout       time.Duration
	stateMu           sync.Mutex
	pendingName       map[int64]struct{}
	pendingInviteCode map[int64]struct{}
}

type menuVariant int

const (
	maxDeviceNameLength = 128
	maxInviteCodeLength = 128
	deviceActionPrefix  = "device"
	deviceActionConfig  = "config"
	deviceActionShowQR  = "qr"
	deviceActionRevoke  = "revoke"
	menuDevicesLabel    = "My devices"
	menuCreateLabel     = "Create device"
	menuInviteCodeLabel = "Enter invite code"
	menuHelpLabel       = "Help"

	menuVariantActive menuVariant = iota
	menuVariantInviteOnly
	menuVariantHelpOnly
)

type telegramClient interface {
	GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]telegram.Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithInlineKeyboard(ctx context.Context, chatID int64, text string, keyboard telegram.InlineKeyboardMarkup) error
	SendMessageWithReplyKeyboard(ctx context.Context, chatID int64, text string, keyboard telegram.ReplyKeyboardMarkup) error
	SendDocument(ctx context.Context, chatID int64, fileName string, document []byte, caption string) error
	AnswerCallbackQuery(ctx context.Context, callbackQueryID, text string) error
	SetCommands(ctx context.Context, commands []telegram.BotCommand) error
}

type backendClient interface {
	backendapi.HealthChecker
	GetAccessStatus(ctx context.Context, telegramUserID int64) (*backendapi.AccessStatusResult, error)
	ApplyInviteCode(ctx context.Context, telegramUserID int64, code string) (*backendapi.AccessStatusResult, error)
	ListDevices(ctx context.Context, telegramUserID int64) (*backendapi.ListDevicesResult, error)
	CreateDevice(ctx context.Context, telegramUserID int64, name string) (*backendapi.CreateDeviceResult, error)
	ResendDeviceConfig(ctx context.Context, telegramUserID, deviceID int64) (*backendapi.ResendDeviceConfigResult, error)
	RevokeDevice(ctx context.Context, telegramUserID, deviceID int64) (*backendapi.RevokeDeviceResult, error)
}

func New(logger *slog.Logger, telegramClient telegramClient, backendClient backendClient, pollTimeout time.Duration) *Bot {
	return &Bot{
		logger:            logger,
		telegram:          telegramClient,
		backend:           backendClient,
		pollTimeout:       pollTimeout,
		pendingName:       make(map[int64]struct{}),
		pendingInviteCode: make(map[int64]struct{}),
	}
}

func (b *Bot) Run(ctx context.Context) error {
	if err := b.syncCommands(ctx); err != nil {
		b.logger.Warn("sync telegram commands", "error", err)
	}

	if err := b.checkBackend(ctx); err != nil {
		b.logger.Warn("backend api is unavailable at startup", "error", err)
	}

	b.logger.Info("telegram bot started")

	var offset int64
	for {
		updates, err := b.telegram.GetUpdates(ctx, offset, b.pollTimeout)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}

			b.logger.Error("poll telegram updates", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}

		for _, update := range updates {
			switch {
			case update.CallbackQuery != nil:
				if err := b.handleCallbackQuery(ctx, update.CallbackQuery); err != nil {
					b.logger.Error("handle telegram callback query", "error", err)
				}
			case update.Message != nil && update.Message.Text != "":
				if err := b.handleMessage(ctx, update.Message); err != nil {
					b.logger.Error("handle telegram message", "error", err)
				}
			}

			offset = update.UpdateID + 1
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, message *telegram.Message) error {
	trimmedText := strings.TrimSpace(message.Text)
	fields := strings.Fields(trimmedText)
	if len(fields) == 0 {
		return nil
	}

	telegramUserID := int64(0)
	if message.From != nil {
		telegramUserID = message.From.ID
	}

	b.logger.Info(
		"handle telegram command",
		"command", sanitizeLoggedInput(trimmedText, fields[0], b.hasPendingInviteCode(message.Chat.ID)),
		"chat_id", message.Chat.ID,
		"telegram_user_id", telegramUserID,
	)

	if menuCommand, ok := normalizeMenuCommand(trimmedText); ok {
		b.clearPendingDeviceName(message.Chat.ID)
		b.clearPendingInviteCode(message.Chat.ID)
		switch menuCommand {
		case "/devices":
			return b.handleDevices(ctx, message)
		case "/newdevice":
			return b.handleNewDevice(ctx, message, "")
		case "/promo":
			return b.handlePromo(ctx, message, "")
		case "/help":
			return b.sendMenuMessage(ctx, message.Chat.ID, helpMessage())
		}
	}

	if strings.HasPrefix(fields[0], "/") {
		if fields[0] != "/newdevice" {
			b.clearPendingDeviceName(message.Chat.ID)
		}
		if fields[0] != "/promo" {
			b.clearPendingInviteCode(message.Chat.ID)
		}
	}

	if b.hasPendingDeviceName(message.Chat.ID) && !strings.HasPrefix(fields[0], "/") {
		return b.handlePendingDeviceName(ctx, message, trimmedText)
	}

	if b.hasPendingInviteCode(message.Chat.ID) && !strings.HasPrefix(fields[0], "/") {
		return b.handlePendingInviteCode(ctx, message, trimmedText)
	}

	switch fields[0] {
	case "/start":
		b.clearPendingDeviceName(message.Chat.ID)
		b.clearPendingInviteCode(message.Chat.ID)
		return b.handleStart(ctx, message)
	case "/help":
		b.clearPendingDeviceName(message.Chat.ID)
		b.clearPendingInviteCode(message.Chat.ID)
		return b.sendMenuMessage(ctx, message.Chat.ID, helpMessage())
	case "/devices":
		return b.handleDevices(ctx, message)
	case "/newdevice":
		return b.handleNewDevice(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	case "/promo":
		return b.handlePromo(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	case "/config":
		return b.handleConfig(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	case "/revoke":
		return b.handleRevoke(ctx, message, strings.TrimSpace(strings.TrimPrefix(message.Text, fields[0])))
	default:
		return nil
	}
}

func (b *Bot) handleStart(ctx context.Context, message *telegram.Message) error {
	if message.From == nil {
		return b.sendMenuMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	result, err := b.backend.GetAccessStatus(ctx, message.From.ID)
	if err != nil {
		b.clearPendingInviteCode(message.Chat.ID)
		b.logger.Error("get access status via backend", "telegram_user_id", message.From.ID, "error", err)
		return b.sendMenuMessage(ctx, message.Chat.ID, "VPN bot is connected, but backend API is temporarily unavailable.\n\nPlease try again in a moment.")
	}

	if shouldPromptInviteCodeEntry(result) {
		b.markPendingInviteCode(message.Chat.ID)
	} else {
		b.clearPendingInviteCode(message.Chat.ID)
	}

	return b.sendAccessAwareMessage(ctx, message.Chat.ID, formatStartAccessMessage(result), result)
}

func helpMessage() string {
	return "Available commands:\n/start - show onboarding and access status\n/help - show this help\n/devices - show your devices and available actions\n/newdevice - create a new device\n/promo <code> - activate invite-only beta access\n\nYou can also use the buttons below."
}

func (b *Bot) checkBackend(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := b.backend.Health(healthCtx); err != nil {
		return fmt.Errorf("check backend health: %w", err)
	}

	return nil
}

func (b *Bot) syncCommands(ctx context.Context) error {
	if b.telegram == nil {
		return nil
	}

	return b.telegram.SetCommands(ctx, closedBetaBotCommands())
}

func closedBetaBotCommands() []telegram.BotCommand {
	return []telegram.BotCommand{
		{
			Command:     "start",
			Description: "Check access and begin",
		},
		{
			Command:     "help",
			Description: "Show help",
		},
		{
			Command:     "promo",
			Description: "Enter invite code",
		},
	}
}

func (b *Bot) handleCallbackQuery(ctx context.Context, callback *telegram.CallbackQuery) error {
	if callback == nil {
		return nil
	}

	telegramUserID := int64(0)
	if callback.From != nil {
		telegramUserID = callback.From.ID
	}

	b.logger.Info(
		"handle telegram callback",
		"data", callback.Data,
		"telegram_user_id", telegramUserID,
	)

	action, deviceID, err := parseDeviceAction(callback.Data)
	if err != nil {
		b.answerCallback(ctx, callback.ID, "This action is no longer available.")
		return nil
	}

	switch action {
	case deviceActionConfig:
		return b.handleConfigCallback(ctx, callback, deviceID)
	case deviceActionShowQR:
		return b.handleQRCodeCallback(ctx, callback, deviceID)
	case deviceActionRevoke:
		return b.handleRevokeCallback(ctx, callback, deviceID)
	default:
		b.answerCallback(ctx, callback.ID, "This action is no longer available.")
		return nil
	}
}

func (b *Bot) handleDevices(ctx context.Context, message *telegram.Message) error {
	if message.From == nil {
		return b.sendMenuMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	result, err := b.backend.ListDevices(ctx, message.From.ID)
	if err != nil {
		if shouldPromptInviteCodeEntryFromError(err) {
			b.markPendingInviteCode(message.Chat.ID)
			return b.sendInvitePromptMessage(ctx, message.Chat.ID, formatListDevicesError(err))
		}
		b.logger.Error("list devices via backend", "telegram_user_id", message.From.ID, "error", err)
		return b.sendMenuMessage(ctx, message.Chat.ID, formatListDevicesError(err))
	}

	return b.sendDevicesList(ctx, message.Chat.ID, result)
}

func (b *Bot) handleNewDevice(ctx context.Context, message *telegram.Message, rawName string) error {
	if message.From == nil {
		return b.sendMenuMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	b.clearPendingInviteCode(message.Chat.ID)

	if strings.TrimSpace(rawName) == "" {
		b.markPendingDeviceName(message.Chat.ID)
		return b.sendMenuMessage(ctx, message.Chat.ID, "Let's spin up a new device.\n\nHow should we call it?\nExamples: iphone, macbook, dad-phone")
	}

	b.clearPendingDeviceName(message.Chat.ID)

	name, validationMessage := validateDeviceName(rawName)
	if validationMessage != "" {
		return b.sendMenuMessage(ctx, message.Chat.ID, validationMessage)
	}

	if err := b.sendMenuMessage(ctx, message.Chat.ID, createDeviceProgressMessage(name)); err != nil {
		return err
	}

	result, err := b.backend.CreateDevice(ctx, message.From.ID, name)
	if err != nil {
		if shouldPromptInviteCodeEntryFromError(err) {
			b.markPendingInviteCode(message.Chat.ID)
			return b.sendInvitePromptMessage(ctx, message.Chat.ID, formatCreateDeviceError(err))
		}
		b.logger.Error("create device via backend", "telegram_user_id", message.From.ID, "device_name", name, "error", err)
		return b.sendMenuMessage(ctx, message.Chat.ID, formatCreateDeviceError(err))
	}

	deviceName := ""
	clientConfig := ""
	if result != nil {
		deviceName = result.Device.Name
		clientConfig = result.ClientConfig
	}

	return b.sendConfigResult(ctx, message.Chat.ID, formatCreateDeviceMessage(result), deviceName, clientConfig)
}

func (b *Bot) handlePromo(ctx context.Context, message *telegram.Message, rawCode string) error {
	if message.From == nil {
		return b.sendMenuMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	b.clearPendingDeviceName(message.Chat.ID)

	if strings.TrimSpace(rawCode) == "" {
		b.markPendingInviteCode(message.Chat.ID)
		return b.sendInvitePromptMessage(ctx, message.Chat.ID, "Enter the invite code exactly as you received it.\n\nYou can also send /promo <code> in one message.")
	}

	b.clearPendingInviteCode(message.Chat.ID)

	code, validationMessage := validateInviteCode(rawCode)
	if validationMessage != "" {
		return b.sendInvitePromptMessage(ctx, message.Chat.ID, validationMessage)
	}

	if err := b.sendInvitePromptMessage(ctx, message.Chat.ID, "Checking your invite code..."); err != nil {
		return err
	}

	result, err := b.backend.ApplyInviteCode(ctx, message.From.ID, code)
	if err != nil {
		b.logger.Error("apply invite code via backend", "telegram_user_id", message.From.ID, "error", err)
		return b.sendInvitePromptMessage(ctx, message.Chat.ID, formatApplyInviteCodeError(err))
	}

	return b.sendMenuMessage(ctx, message.Chat.ID, formatApplyInviteCodeSuccess(result))
}

func (b *Bot) handleConfig(ctx context.Context, message *telegram.Message, rawDeviceID string) error {
	if message.From == nil {
		return b.sendMenuMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	deviceID, validationMessage := parseDeviceID(rawDeviceID, "/config <device_id>")
	if validationMessage != "" {
		return b.sendMenuMessage(ctx, message.Chat.ID, validationMessage)
	}

	result, err := b.backend.ResendDeviceConfig(ctx, message.From.ID, deviceID)
	if err != nil {
		b.logger.Error("resend device config via backend", "telegram_user_id", message.From.ID, "device_id", deviceID, "error", err)
		return b.sendMenuMessage(ctx, message.Chat.ID, formatResendDeviceConfigError(err))
	}

	deviceName := ""
	clientConfig := ""
	if result != nil {
		deviceName = result.Device.Name
		clientConfig = result.ClientConfig
	}

	return b.sendConfigResult(ctx, message.Chat.ID, formatResendDeviceConfigMessage(result), deviceName, clientConfig)
}

func (b *Bot) handleRevoke(ctx context.Context, message *telegram.Message, rawDeviceID string) error {
	if message.From == nil {
		return b.sendMenuMessage(ctx, message.Chat.ID, "Cannot identify Telegram user for this command.")
	}

	deviceID, validationMessage := parseDeviceID(rawDeviceID, "/revoke <device_id>")
	if validationMessage != "" {
		return b.sendMenuMessage(ctx, message.Chat.ID, validationMessage)
	}

	result, err := b.backend.RevokeDevice(ctx, message.From.ID, deviceID)
	if err != nil {
		b.logger.Error("revoke device via backend", "telegram_user_id", message.From.ID, "device_id", deviceID, "error", err)
		return b.sendMenuMessage(ctx, message.Chat.ID, formatRevokeDeviceError(err))
	}

	return b.sendMenuMessage(ctx, message.Chat.ID, formatRevokeDeviceMessage(result))
}

func (b *Bot) handleConfigCallback(ctx context.Context, callback *telegram.CallbackQuery, deviceID int64) error {
	chatID, telegramUserID, ok := callbackContext(callback)
	if !ok {
		b.answerCallback(ctx, callback.ID, "This action is no longer available.")
		return nil
	}

	b.answerCallback(ctx, callback.ID, configProgressMessage())

	result, err := b.backend.ResendDeviceConfig(ctx, telegramUserID, deviceID)
	if err != nil {
		b.logger.Error("resend device config via callback", "telegram_user_id", telegramUserID, "device_id", deviceID, "error", err)
		message := formatResendDeviceConfigError(err)
		return b.sendMenuMessage(ctx, chatID, message)
	}

	return b.sendConfigMessage(ctx, chatID, formatResendDeviceConfigMessage(result))
}

func (b *Bot) handleQRCodeCallback(ctx context.Context, callback *telegram.CallbackQuery, deviceID int64) error {
	chatID, telegramUserID, ok := callbackContext(callback)
	if !ok {
		b.answerCallback(ctx, callback.ID, "This action is no longer available.")
		return nil
	}

	b.answerCallback(ctx, callback.ID, qrProgressMessage())

	result, err := b.backend.ResendDeviceConfig(ctx, telegramUserID, deviceID)
	if err != nil {
		b.logger.Error("resend device qr via callback", "telegram_user_id", telegramUserID, "device_id", deviceID, "error", err)
		message := formatResendDeviceConfigError(err)
		return b.sendMenuMessage(ctx, chatID, message)
	}

	deviceName := ""
	clientConfig := ""
	if result != nil {
		deviceName = result.Device.Name
		clientConfig = result.ClientConfig
	}

	return b.sendQRCodeResult(ctx, chatID, deviceName, clientConfig)
}

func (b *Bot) handleRevokeCallback(ctx context.Context, callback *telegram.CallbackQuery, deviceID int64) error {
	chatID, telegramUserID, ok := callbackContext(callback)
	if !ok {
		b.answerCallback(ctx, callback.ID, "This action is no longer available.")
		return nil
	}

	b.answerCallback(ctx, callback.ID, revokeProgressMessage())

	result, err := b.backend.RevokeDevice(ctx, telegramUserID, deviceID)
	if err != nil {
		b.logger.Error("revoke device via callback", "telegram_user_id", telegramUserID, "device_id", deviceID, "error", err)
		message := formatRevokeDeviceError(err)
		return b.sendMenuMessage(ctx, chatID, message)
	}

	return b.sendMenuMessage(ctx, chatID, formatRevokeDeviceMessage(result))
}

func (b *Bot) sendDevicesList(ctx context.Context, chatID int64, result *backendapi.ListDevicesResult) error {
	activeDevices := visibleDevices(result)

	if err := b.sendMenuMessage(ctx, chatID, formatDevicesSummary(result, activeDevices)); err != nil {
		return err
	}

	if len(activeDevices) == 0 {
		return nil
	}

	for _, device := range activeDevices {
		keyboard := deviceActionsKeyboard(device)
		if keyboard != nil {
			if err := b.telegram.SendMessageWithInlineKeyboard(ctx, chatID, formatDeviceCard(device), *keyboard); err != nil {
				return err
			}
			continue
		}

		if err := b.sendMenuMessage(ctx, chatID, formatDeviceCard(device)); err != nil {
			return err
		}
	}

	return nil
}

func visibleDevices(result *backendapi.ListDevicesResult) []backendapi.Device {
	if result == nil || len(result.Devices) == 0 {
		return nil
	}

	devices := make([]backendapi.Device, 0, len(result.Devices))
	for _, device := range result.Devices {
		if device.Status == "revoked" {
			continue
		}

		devices = append(devices, device)
	}

	return devices
}

func formatDevicesSummary(result *backendapi.ListDevicesResult, activeDevices []backendapi.Device) string {
	if result == nil || len(result.Devices) == 0 {
		return "You have no devices yet.\n\nTap Create device below to add one."
	}

	if len(activeDevices) == 0 {
		return "You have no active devices right now.\n\nTap Create device below to add a new one."
	}

	return "Your devices:\n\nTap a button under any active device to get the config, show the QR, or revoke it."
}

func formatDeviceCard(device backendapi.Device) string {
	lines := []string{fmt.Sprintf("Name: %s", device.Name)}
	lines = append(lines, fmt.Sprintf("Status: %s", formatDeviceStatus(device.Status)))

	if device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", device.AssignedIP))
	}

	if !device.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", device.CreatedAt.Format("2006-01-02")))
	}

	if device.RevokedAt != nil {
		lines = append(lines, fmt.Sprintf("Revoked: %s", device.RevokedAt.Format("2006-01-02")))
	}

	return strings.Join(lines, "\n")
}

func formatDeviceStatus(status string) string {
	switch status {
	case "active":
		return "Active"
	case "revoked":
		return "Revoked"
	default:
		if status == "" {
			return "Unknown"
		}

		return strings.ToUpper(status[:1]) + status[1:]
	}
}

func deviceActionsKeyboard(device backendapi.Device) *telegram.InlineKeyboardMarkup {
	if device.ID <= 0 || device.Status == "revoked" {
		return nil
	}

	return &telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{
					Text:         "Get config",
					CallbackData: deviceActionData(deviceActionConfig, device.ID),
				},
				{
					Text:         "Show QR",
					CallbackData: deviceActionData(deviceActionShowQR, device.ID),
				},
			},
			{
				{
					Text:         "Revoke",
					CallbackData: deviceActionData(deviceActionRevoke, device.ID),
				},
			},
		},
	}
}

func formatCreateDeviceMessage(result *backendapi.CreateDeviceResult) string {
	if result == nil {
		return "Device created."
	}

	lines := []string{"Device created successfully."}
	lines = append(lines, fmt.Sprintf("Name: %s", result.Device.Name))

	if result.Device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", result.Device.AssignedIP))
	}

	if result.Device.Status != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", result.Device.Status))
	}

	if !result.Device.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", result.Device.CreatedAt.Format("2006-01-02")))
	}

	if strings.TrimSpace(result.ClientConfig) != "" {
		lines = append(lines, "", "Client config:", result.ClientConfig)
	}

	return strings.Join(lines, "\n")
}

func createDeviceProgressMessage(name string) string {
	return fmt.Sprintf("Rick is calibrating the portal gun for %s.\n\nBuilding your WireGuard config...", name)
}

func configProgressMessage() string {
	return "Morty, hold still. Rebuilding that config."
}

func qrProgressMessage() string {
	return "Opening a tiny portal and turning it into a QR code."
}

func revokeProgressMessage() string {
	return "Rick is closing this portal and cleaning up the timeline."
}

func formatCreateDeviceError(err error) string {
	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 0:
			return "Failed to create the device right now. Please try again later."
		case apiErr.StatusCode == 400 && apiErr.Message != "":
			return fmt.Sprintf("Cannot create device: %s.", apiErr.Message)
		case apiErr.StatusCode == 403:
			return formatInactiveAccessPrompt()
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			return fmt.Sprintf("Cannot create device: %s.", apiErr.Message)
		default:
			return "Failed to create the device right now. Please try again later."
		}
	}

	return "Failed to create the device right now. Please try again later."
}

func formatListDevicesError(err error) string {
	if errors.Is(err, backendapi.ErrNotFound) {
		return "You are not linked to a VPN user yet."
	}

	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 403:
			return formatInactiveAccessPrompt()
		default:
			return "Failed to load devices right now. Please try again later."
		}
	}

	return "Failed to load devices right now. Please try again later."
}

func formatResendDeviceConfigMessage(result *backendapi.ResendDeviceConfigResult) string {
	if result == nil {
		return "Device config rebuilt."
	}

	lines := []string{"Device config rebuilt successfully."}
	if result.Device.Name != "" {
		lines = append(lines, fmt.Sprintf("Name: %s", result.Device.Name))
	}
	if result.Device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", result.Device.AssignedIP))
	}
	if strings.TrimSpace(result.ClientConfig) != "" {
		lines = append(lines, "", "Client config:", result.ClientConfig)
	}

	return strings.Join(lines, "\n")
}

func formatResendDeviceConfigError(err error) string {
	if errors.Is(err, backendapi.ErrNotFound) {
		return "Device not found."
	}

	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 403:
			return "You are not allowed to access that device config."
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			return fmt.Sprintf("Cannot resend config: %s.", apiErr.Message)
		default:
			return "Failed to resend the device config right now. Please try again later."
		}
	}

	return "Failed to resend the device config right now. Please try again later."
}

func formatRevokeDeviceMessage(result *backendapi.RevokeDeviceResult) string {
	if result == nil {
		return "Device revoked."
	}

	lines := []string{"Device revoked successfully."}
	if result.Device.Name != "" {
		lines = append(lines, fmt.Sprintf("Name: %s", result.Device.Name))
	}
	if result.Device.AssignedIP != "" {
		lines = append(lines, fmt.Sprintf("IP: %s", result.Device.AssignedIP))
	}
	if result.Device.Status != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", result.Device.Status))
	}
	if result.Device.RevokedAt != nil {
		lines = append(lines, fmt.Sprintf("Revoked: %s", result.Device.RevokedAt.Format("2006-01-02")))
	}

	return strings.Join(lines, "\n")
}

func formatRevokeDeviceError(err error) string {
	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 403:
			return "You are not allowed to revoke that device."
		case apiErr.StatusCode == 404:
			return "Device not found or unavailable."
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			return fmt.Sprintf("Cannot revoke device: %s.", apiErr.Message)
		default:
			return "Failed to revoke the device right now. Please try again later."
		}
	}

	return "Failed to revoke the device right now. Please try again later."
}

func callbackContext(callback *telegram.CallbackQuery) (chatID, telegramUserID int64, ok bool) {
	if callback == nil || callback.Message == nil || callback.From == nil {
		return 0, 0, false
	}

	return callback.Message.Chat.ID, callback.From.ID, true
}

func (b *Bot) answerCallback(ctx context.Context, callbackQueryID, text string) {
	if callbackQueryID == "" {
		return
	}

	if err := b.telegram.AnswerCallbackQuery(ctx, callbackQueryID, text); err != nil {
		b.logger.Warn("answer callback query", "callback_query_id", callbackQueryID, "error", err)
	}
}

func (b *Bot) sendMenuMessage(ctx context.Context, chatID int64, text string) error {
	return b.sendMenuMessageWithKeyboard(ctx, chatID, text, menuKeyboard(menuVariantActive))
}

func (b *Bot) sendInvitePromptMessage(ctx context.Context, chatID int64, text string) error {
	return b.sendMenuMessageWithKeyboard(ctx, chatID, text, menuKeyboard(menuVariantInviteOnly))
}

func (b *Bot) sendAccessAwareMessage(ctx context.Context, chatID int64, text string, result *backendapi.AccessStatusResult) error {
	return b.sendMenuMessageWithKeyboard(ctx, chatID, text, keyboardForAccessStatus(result))
}

func (b *Bot) sendMenuMessageWithKeyboard(ctx context.Context, chatID int64, text string, keyboard telegram.ReplyKeyboardMarkup) error {
	return b.telegram.SendMessageWithReplyKeyboard(ctx, chatID, text, keyboard)
}

func (b *Bot) sendConfigMessage(ctx context.Context, chatID int64, text string) error {
	return b.sendMenuMessage(ctx, chatID, text)
}

func (b *Bot) sendConfigResult(ctx context.Context, chatID int64, text, deviceName, clientConfig string) error {
	if err := b.sendConfigMessage(ctx, chatID, text); err != nil {
		return err
	}

	return b.sendQRCodeDocument(ctx, chatID, deviceName, clientConfig, "QR code is unavailable right now. Use the config text above.")
}

func (b *Bot) sendQRCodeResult(ctx context.Context, chatID int64, deviceName, clientConfig string) error {
	return b.sendQRCodeDocument(ctx, chatID, deviceName, clientConfig, "QR code is unavailable right now. Use Get config to see the raw config text.")
}

func (b *Bot) sendQRCodeDocument(ctx context.Context, chatID int64, deviceName, clientConfig, fallbackMessage string) error {
	qrPNG, err := botqrcode.GeneratePNG(clientConfig)
	if err != nil {
		b.logger.Error("generate qr code", "device_name", deviceName, "error", err)
		return b.sendMenuMessage(ctx, chatID, fallbackMessage)
	}

	fileName := "wireguard-config-qr.png"
	if deviceName != "" {
		fileName = sanitizeFileName(deviceName) + "-wireguard-qr.png"
	}

	if err := b.telegram.SendDocument(ctx, chatID, fileName, qrPNG, "WireGuard QR code"); err != nil {
		b.logger.Error("send qr code", "device_name", deviceName, "error", err)
		return b.sendMenuMessage(ctx, chatID, fallbackMessage)
	}

	return nil
}

func validateDeviceName(rawName string) (string, string) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return "", "Usage: /newdevice <device_name>"
	}

	if utf8.RuneCountInString(name) > maxDeviceNameLength {
		return "", fmt.Sprintf("Device name is too long. Use %d characters or fewer.", maxDeviceNameLength)
	}

	for _, r := range name {
		if unicode.IsControl(r) {
			return "", "Device name must be a single line of plain text."
		}
	}

	return name, ""
}

func validateInviteCode(rawCode string) (string, string) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return "", "Usage: /promo <invite_code>"
	}

	if utf8.RuneCountInString(code) > maxInviteCodeLength {
		return "", fmt.Sprintf("Invite code is too long. Use %d characters or fewer.", maxInviteCodeLength)
	}

	for _, r := range code {
		if unicode.IsControl(r) {
			return "", "Invite code must be a single line of plain text."
		}
	}

	return code, ""
}

func parseDeviceID(rawValue, usage string) (int64, string) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return 0, "Usage: " + usage
	}

	deviceID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || deviceID <= 0 {
		return 0, "Device ID must be a positive number."
	}

	return deviceID, ""
}

func normalizeMenuCommand(text string) (string, bool) {
	switch strings.TrimSpace(text) {
	case menuDevicesLabel:
		return "/devices", true
	case menuCreateLabel:
		return "/newdevice", true
	case menuInviteCodeLabel:
		return "/promo", true
	case menuHelpLabel:
		return "/help", true
	default:
		return "", false
	}
}

func sanitizeLoggedInput(rawText, firstField string, pendingInviteCode bool) string {
	if pendingInviteCode && !strings.HasPrefix(firstField, "/") {
		return "<invite_code_redacted>"
	}

	if firstField == "/promo" {
		if strings.TrimSpace(strings.TrimPrefix(rawText, firstField)) == "" {
			return "/promo"
		}

		return "/promo <invite_code_redacted>"
	}

	return rawText
}

func menuKeyboard(variant menuVariant) telegram.ReplyKeyboardMarkup {
	keyboard := telegram.ReplyKeyboardMarkup{
		ResizeKeyboard:        true,
		IsPersistent:          true,
		InputFieldPlaceholder: "Choose an action",
	}

	switch variant {
	case menuVariantInviteOnly:
		keyboard.Keyboard = [][]telegram.KeyboardButton{
			{
				{Text: menuInviteCodeLabel},
				{Text: menuHelpLabel},
			},
		}
	case menuVariantHelpOnly:
		keyboard.Keyboard = [][]telegram.KeyboardButton{
			{
				{Text: menuHelpLabel},
			},
		}
	default:
		keyboard.Keyboard = [][]telegram.KeyboardButton{
			{
				{Text: menuDevicesLabel},
				{Text: menuCreateLabel},
			},
			{
				{Text: menuInviteCodeLabel},
				{Text: menuHelpLabel},
			},
		}
	}

	return keyboard
}

func keyboardForAccessStatus(result *backendapi.AccessStatusResult) telegram.ReplyKeyboardMarkup {
	if result == nil || result.AccessActive {
		return menuKeyboard(menuVariantActive)
	}

	switch result.DenialReason {
	case "user_blocked", "user_deleted":
		return menuKeyboard(menuVariantHelpOnly)
	default:
		return menuKeyboard(menuVariantInviteOnly)
	}
}

func formatStartAccessMessage(result *backendapi.AccessStatusResult) string {
	if result == nil {
		return "VPN bot is connected.\n\nUse the buttons below to continue."
	}

	if result.AccessActive {
		return formatActiveAccessMessage(result)
	}

	return formatInactiveAccessMessage(result)
}

func formatActiveAccessMessage(result *backendapi.AccessStatusResult) string {
	if result == nil {
		return "Access is active.\n\nUse the buttons below to manage devices or create a new one."
	}

	if result.IsLifetime {
		return "Access is active.\n\nClosed beta access is lifetime.\nUse the buttons below to manage devices or create a new one."
	}

	if result.ExpiresAt != nil {
		return fmt.Sprintf("Access is active.\n\nYour beta access is active until %s.\nUse the buttons below to manage devices or create a new one.", result.ExpiresAt.Format("2006-01-02"))
	}

	return "Access is active.\n\nUse the buttons below to manage devices or create a new one."
}

func formatInactiveAccessMessage(result *backendapi.AccessStatusResult) string {
	switch {
	case result == nil:
		return formatInactiveAccessPrompt()
	case result.DenialReason == "expired":
		return "Your beta access has expired.\n\nEnter a new invite code to continue."
	case result.DenialReason == "pending":
		return "Your access is not active yet.\n\nEnter your invite code to continue onboarding."
	case result.DenialReason == "canceled":
		return "Your access is inactive right now.\n\nEnter an invite code to restore access."
	case result.DenialReason == "user_blocked":
		return "Access is currently blocked.\n\nIf this is unexpected, contact the operator."
	case result.DenialReason == "user_deleted":
		return "This account is inactive right now.\n\nIf this is unexpected, contact the operator."
	default:
		return "Closed beta access is not active yet.\n\nEnter your invite code to unlock VPN access."
	}
}

func formatInactiveAccessPrompt() string {
	return "Access is not active yet.\n\nTap Enter invite code below or send /promo <invite_code>."
}

func shouldPromptInviteCodeEntry(result *backendapi.AccessStatusResult) bool {
	if result == nil || result.AccessActive {
		return false
	}

	switch result.DenialReason {
	case "user_blocked", "user_deleted":
		return false
	default:
		return true
	}
}

func shouldPromptInviteCodeEntryFromError(err error) bool {
	var apiErr *backendapi.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 403
}

func formatApplyInviteCodeSuccess(result *backendapi.AccessStatusResult) string {
	if result == nil || !result.AccessActive {
		return "Invite code applied.\n\nAccess details were updated."
	}

	if result.IsLifetime {
		return "Invite code accepted.\n\nLifetime beta access is now active.\nYou can create your first device."
	}

	if result.ExpiresAt != nil {
		return fmt.Sprintf("Invite code accepted.\n\nAccess is active until %s.\nYou can create your first device.", result.ExpiresAt.Format("2006-01-02"))
	}

	return "Invite code accepted.\n\nAccess is active now.\nYou can create your first device."
}

func formatApplyInviteCodeError(err error) string {
	if errors.Is(err, backendapi.ErrNotFound) {
		return "That invite code is not valid."
	}

	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 404:
			return "That invite code is not valid."
		case apiErr.StatusCode == 400 && apiErr.Message != "":
			return fmt.Sprintf("Cannot apply invite code: %s.", apiErr.Message)
		case apiErr.StatusCode == 403:
			return "This account cannot activate access right now."
		case apiErr.StatusCode == 409 && apiErr.Message != "":
			switch apiErr.Message {
			case "promo code is inactive":
				return "That invite code is inactive or expired."
			case "promo code already used":
				return "You have already used that invite code."
			case "promo code usage limit reached":
				return "That invite code has already been fully used."
			case "promo code does not grant access":
				return "That code does not unlock closed beta access."
			default:
				return fmt.Sprintf("Cannot apply invite code: %s.", apiErr.Message)
			}
		default:
			return "Failed to apply the invite code right now. Please try again later."
		}
	}

	return "Failed to apply the invite code right now. Please try again later."
}

func (b *Bot) markPendingInviteCode(chatID int64) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	b.pendingInviteCode[chatID] = struct{}{}
}

func (b *Bot) clearPendingInviteCode(chatID int64) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	delete(b.pendingInviteCode, chatID)
}

func (b *Bot) hasPendingInviteCode(chatID int64) bool {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	_, ok := b.pendingInviteCode[chatID]
	return ok
}

func (b *Bot) handlePendingInviteCode(ctx context.Context, message *telegram.Message, rawCode string) error {
	code, validationMessage := validateInviteCode(rawCode)
	if validationMessage != "" {
		return b.sendMenuMessage(ctx, message.Chat.ID, validationMessage)
	}

	return b.handlePromo(ctx, message, code)
}

func (b *Bot) markPendingDeviceName(chatID int64) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	b.pendingName[chatID] = struct{}{}
}

func (b *Bot) clearPendingDeviceName(chatID int64) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	delete(b.pendingName, chatID)
}

func (b *Bot) hasPendingDeviceName(chatID int64) bool {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	_, ok := b.pendingName[chatID]
	return ok
}

func (b *Bot) handlePendingDeviceName(ctx context.Context, message *telegram.Message, rawName string) error {
	name, validationMessage := validateDeviceName(rawName)
	if validationMessage != "" {
		return b.sendMenuMessage(ctx, message.Chat.ID, validationMessage)
	}

	return b.handleNewDevice(ctx, message, name)
}

func deviceActionData(action string, deviceID int64) string {
	return fmt.Sprintf("%s:%s:%d", deviceActionPrefix, action, deviceID)
}

func parseDeviceAction(value string) (string, int64, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != deviceActionPrefix {
		return "", 0, fmt.Errorf("invalid device action")
	}

	deviceID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || deviceID <= 0 {
		return "", 0, fmt.Errorf("invalid device id")
	}

	return parts[1], deviceID, nil
}

func sanitizeFileName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "device"
	}

	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteByte('-')
		}
	}

	if builder.Len() == 0 {
		return "device"
	}

	return builder.String()
}
