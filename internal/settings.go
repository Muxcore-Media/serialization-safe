package internal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Muxcore-Media/core/pkg/contracts"
)

func (m *Module) Settings() []contracts.SettingDef {
	return m.settingsDefs()
}

func (m *Module) UpdateSetting(key, value string) error {
	return m.updateSetting(key, value)
}

func (m *Module) settingsDefs() []contracts.SettingDef {
	m.mu.RLock()
	maxBytes := m.maxPayloadBytes
	m.mu.RUnlock()
	return []contracts.SettingDef{
		{
			Key:         "max_payload_bytes",
			Label:       "Max Payload Bytes",
			Type:        contracts.SettingTypeInt,
			Value:       strconv.Itoa(maxBytes),
			Default:     strconv.Itoa(defaultMaxPayloadBytes),
			Description: "Reject Convert payloads larger than this size (SERIALIZATION_MAX_PAYLOAD_BYTES)",
			Group:       "Limits",
		},
	}
}

func (m *Module) updateSetting(key, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "max_payload_bytes", "SERIALIZATION_MAX_PAYLOAD_BYTES":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("max_payload_bytes must be a positive integer")
		}
		m.mu.Lock()
		m.maxPayloadBytes = n
		m.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}
