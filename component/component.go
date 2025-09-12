package component

import (
	"encoding/json"
	"strings"
)

type ChatComponent struct {
	Text      string          `json:"text"`
	With      []ChatComponent `json:"with"`
	Translate string          `json:"translate"`
	Extra     []ChatComponent `json:"extra"`
}

func (c *ChatComponent) UnmarshalJSON(data []byte) error {
	type Alias ChatComponent

	aux := &struct {
		With  []json.RawMessage `json:"with"`
		Extra []json.RawMessage `json:"extra"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	for _, raw := range aux.With {
		var comp ChatComponent
		if err := json.Unmarshal(raw, &comp); err == nil {
			c.With = append(c.With, comp)
		} else {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				c.With = append(c.With, ChatComponent{Text: s})
			}
		}
	}

	for _, raw := range aux.Extra {
		var comp ChatComponent
		if err := json.Unmarshal(raw, &comp); err == nil {
			c.Extra = append(c.Extra, comp)
		}
	}

	return nil
}

func (c *ChatComponent) String() string {
	var sb strings.Builder
	sb.WriteString(c.Text)

	for _, part := range c.With {
		sb.WriteString(part.String())
	}

	for _, extra := range c.Extra {
		sb.WriteString(extra.String())
	}
	return sb.String()
}
