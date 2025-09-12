package gophermc

import (
	"github.com/SyNdicateFoundation/GopherMc/component"
	"time"
)

type Event interface{}

type ReadyEvent struct {
	Event
	Username string
}

type DisconnectEvent struct {
	Event
	Reason string
}

type KeepAliveEvent struct {
	Event
	ID int64
}

type ChatMessageEvent struct {
	Event
	Message   string
	Component component.ChatComponent
	Sender    string
	Time      time.Time
}
