package notifications

import "github.com/deckarep/gosx-notifier"

// OSXNotifier implements the Notifier interface for OSX/Darwin
type OSXNotifier struct{}

// Push sends a notification
func (notifier OSXNotifier) Push(n Notification) error {
	note := gosxnotifier.NewNotification(n.Content)
	note.Title = n.Title
	note.AppIcon = n.Icon
	return note.Push()
}
