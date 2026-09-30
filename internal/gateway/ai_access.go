package gateway

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wolfbbs/internal/domain"
	"wolfbbs/internal/rbac"
)

// System setting keys for who may use the AI door and how often. Sysops
// always have access; everyone else needs a grant from the sysop, so a board
// that switches the AI door on doesn't hand strangers a paid (or local)
// model by default.
const (
	SettingAIAllowedUsers = "gateway.ai.allowed_users"
	SettingAIDailyCap     = "gateway.ai.daily_cap"
	SettingAINoThinking   = "gateway.ai.no_thinking"
	settingAIUsagePrefix  = "gateway.ai.usage."

	DefaultAIDailyCap = 20
)

// AISettingsStore is the slice of the admin repository the AI policy needs.
type AISettingsStore interface {
	GetSystemSetting(key string) (string, error)
	UpsertSystemSetting(key, value string) error
}

type AIPolicy struct {
	AllowedHandles []string
	DailyCap       int // 0 means no cap for granted users
	NoThinking     bool
}

type AIAccess struct {
	Allowed   bool
	Unlimited bool
	Used      int
	Remaining int
	Reason    string
}

// usageMu serialises read-modify-write of usage counters within one process.
var usageMu sync.Mutex

func LoadAIPolicy(store AISettingsStore) AIPolicy {
	policy := AIPolicy{DailyCap: DefaultAIDailyCap}
	if store == nil {
		return policy
	}
	if raw, err := store.GetSystemSetting(SettingAIAllowedUsers); err == nil {
		policy.AllowedHandles = ParseAIAllowedHandles(raw)
	}
	if raw, err := store.GetSystemSetting(SettingAIDailyCap); err == nil && strings.TrimSpace(raw) != "" {
		if n, convErr := strconv.Atoi(strings.TrimSpace(raw)); convErr == nil && n >= 0 {
			policy.DailyCap = n
		}
	}
	if raw, err := store.GetSystemSetting(SettingAINoThinking); err == nil {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "true", "on", "yes":
			policy.NoThinking = true
		}
	}
	return policy
}

func SaveAIPolicy(store AISettingsStore, policy AIPolicy) error {
	if store == nil {
		return fmt.Errorf("settings store unavailable")
	}
	if policy.DailyCap < 0 {
		policy.DailyCap = 0
	}
	if err := store.UpsertSystemSetting(SettingAIAllowedUsers, FormatAIAllowedHandles(policy.AllowedHandles)); err != nil {
		return err
	}
	if err := store.UpsertSystemSetting(SettingAIDailyCap, strconv.Itoa(policy.DailyCap)); err != nil {
		return err
	}
	return store.UpsertSystemSetting(SettingAINoThinking, strconv.FormatBool(policy.NoThinking))
}

// ParseAIAllowedHandles accepts handles separated by commas, spaces or newlines.
func ParseAIAllowedHandles(raw string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
	}) {
		handle := strings.ToLower(strings.TrimSpace(field))
		if handle == "" || seen[handle] {
			continue
		}
		seen[handle] = true
		out = append(out, handle)
	}
	sort.Strings(out)
	return out
}

func FormatAIAllowedHandles(handles []string) string {
	return strings.Join(ParseAIAllowedHandles(strings.Join(handles, ",")), ", ")
}

func (p AIPolicy) grants(handle string) bool {
	handle = strings.ToLower(strings.TrimSpace(handle))
	for _, h := range p.AllowedHandles {
		if h == handle {
			return true
		}
	}
	return false
}

func isAISysop(user *domain.User) bool {
	return user != nil && rbac.NormalizeRole(user.Role) == rbac.RoleSysop
}

// CheckAIAccess reports whether user may send an AI prompt right now.
func CheckAIAccess(store AISettingsStore, user *domain.User, now time.Time) AIAccess {
	if user == nil {
		return AIAccess{Reason: "Log in to use the AI assistant."}
	}
	if isAISysop(user) {
		return AIAccess{Allowed: true, Unlimited: true}
	}
	policy := LoadAIPolicy(store)
	if !policy.grants(user.Handle) {
		return AIAccess{Reason: "The AI assistant is open to members the sysop has granted access. Ask the sysop if you'd like it."}
	}
	if policy.DailyCap == 0 {
		return AIAccess{Allowed: true, Unlimited: true}
	}
	used := aiUsageToday(store, user.ID, now)
	remaining := policy.DailyCap - used
	if remaining <= 0 {
		return AIAccess{Used: used, Reason: fmt.Sprintf("You've used all %d of today's AI prompts. They reset tomorrow.", policy.DailyCap)}
	}
	return AIAccess{Allowed: true, Used: used, Remaining: remaining}
}

// RecordAIUse counts one successful prompt against user's daily cap.
// Sysops are not counted.
func RecordAIUse(store AISettingsStore, user *domain.User, now time.Time) error {
	if store == nil || user == nil || isAISysop(user) {
		return nil
	}
	usageMu.Lock()
	defer usageMu.Unlock()
	used := aiUsageToday(store, user.ID, now)
	return store.UpsertSystemSetting(aiUsageKey(user.ID), aiDay(now)+":"+strconv.Itoa(used+1))
}

func aiUsageKey(userID int64) string {
	return settingAIUsagePrefix + strconv.FormatInt(userID, 10)
}

func aiDay(now time.Time) string {
	return now.Format("2006-01-02")
}

func aiUsageToday(store AISettingsStore, userID int64, now time.Time) int {
	if store == nil {
		return 0
	}
	raw, err := store.GetSystemSetting(aiUsageKey(userID))
	if err != nil {
		return 0
	}
	day, count, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok || day != aiDay(now) {
		return 0
	}
	n, err := strconv.Atoi(count)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
