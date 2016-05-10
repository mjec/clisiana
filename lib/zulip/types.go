package zulip

import (
	"encoding/json"
	"fmt"
)

// Context is a Zupli API context (authentication details)
type Context struct {
	Email   string
	APIKey  string
	APIBase string
	Secure  bool
}

// EventType is the type of Zulip API event
type EventType int

const (
	// MessageEvent represents an incoming message event
	MessageEvent EventType = 1 << iota
	// SubscriptionsEvent represents an event for a change in the user's subscriptions
	SubscriptionsEvent EventType = 1 << iota
	// RealmUserEvent represents a realm user event
	RealmUserEvent EventType = 1 << iota
	// PointerEvent represents a change in the user's pointer
	PointerEvent EventType = 1 << iota
)

// MessageType is either StreamMessage or PrivateMessage
type MessageType int

const (
	StreamMessage  MessageType = iota // StreamMessage is a message to a stream
	PrivateMessage MessageType = iota // PrivateMessage is a message to one or more individual users
)

// UnmarshalJSON enables MessageType to be decoded from JSON
func (t MessageType) UnmarshalJSON(b []byte) (err error) {
	typeStr := ""
	err = json.Unmarshal(b, &typeStr)
	if err != nil {
		return err
	}
	switch typeStr {
	case "stream":
		t = StreamMessage
	case "private":
		t = PrivateMessage
	default:
		return fmt.Errorf("Unknown message type when attempting to decode from JSON")
	}
	return
}

// Event is a structure for generic events
type Event struct {
	ID            uint64      `json:"id"`
	Type          EventType   `json:"type"`
	Message       Message     `json:"message,omitempty"`
	Subscriptions interface{} `json:"subscriptions,omitempty"`
	RealmUser     interface{} `json:"realm_user,omitempty"`
	Pointer       interface{} `json:"pointer,omitempty"`
}

// User is a structure for Zulip users
type User struct {
	ID            uint64 `json:"id"`
	FullName      string `json:"full_name"`
	Domain        string `json:"domain"`
	Email         string `json:"email"`
	ShortName     string `json:"short_name"`
	IsMirrorDummy bool   `json:"is_mirror_dummy"`
}

// Message is the main message object that is retreived from Zulip
type Message struct {
	ID               uint64           `json:"id"`                // e.g. 12345678
	ContentType      string           `json:"content_type"`      // e.g. 'text/x-markdown'
	AvatarURL        string           `json:"avatar_url"`        // e.g. 'https://url/for/othello-bots/avatar'
	Timestamp        uint64           `json:"timestamp"`         // e.g. 1375978403
	DisplayRecipient DisplayRecipient `json:"display_recipient"` // string or []User, see DisplayRecipient
	SenderID         uint64           `json:"sender_id"`         // e.g. 13215
	SenderFullName   string           `json:"sender_full_name"`  // e.g. 'Othello Bot'
	SenderEmail      string           `json:"sender_email"`      // e.g. 'othello-bot@example.com'
	SenderShortName  string           `json:"sender_short_name"` // e.g. 'othello-bot'
	SenderDomain     string           `json:"sender_domain"`     // e.g. 'example.com'
	Content          string           `json:"content"`           // e.g. 'Something is rotten in the state of Denmark.'
	GravatarHash     string           `json:"gravatar_hash"`     // e.g. '17d93357cca1e793739836ecbc7a9bf7'
	RecipientID      uint64           `json:"recipient_id"`      // e.g. 12314
	Client           string           `json:"client"`            // e.g. 'website'
	SubjectLinks     []interface{}    `json:"subject_links"`     // e.g. []
	Subject          string           `json:"subject"`           // e.g. 'Castle'
	Type             MessageType      `json:"type"`              // e.g. 'stream'
}

// DisplayRecipient is a structure which contains either an array of Users or a Topic
type DisplayRecipient struct {
	Users []User `json:"users,omitempty"`
	Topic string `json:"topic,omitempty"`
}

// UnmarshalJSON enables DisplayRecipient to be decoded from JSON
func (d *DisplayRecipient) UnmarshalJSON(b []byte) (err error) {
	topic, users := "", make([]User, 1)
	if err = json.Unmarshal(b, &topic); err == nil {
		// 'Denmark'
		d.Topic = topic
		return
	}
	if err = json.Unmarshal(b, &users); err == nil {
		// [{'full_name': 'Hamlet of Denmark', 'domain': 'example.com', 'email': 'hamlet@example.com', 'short_name': 'hamlet', 'id': 31572}],
		d.Users = users
		return
	}
	return
}

type zulipSendMessageReturn struct {
	ID      uint64 `json:"id,omitempty"`
	Message string `json:"msg"`
	Result  string `json:"result"`
}

type zulipRegisterReturn struct {
	QueueID     uint64 `json:"queue_id,omitempty"`
	LastEventID uint64 `json:"last_event_id,omitempty"`
	Message     string `json:"msg"`
	Result      string `json:"result"`
}

type zulipEventsReturn struct {
	Message string  `json:"msg"`
	Result  string  `json:"result"`
	Events  []Event `json:"events,omitempty"`
}

const zulipSuccessResult = "success"
