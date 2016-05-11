package notifications

import "os/exec"

// LinuxNotifier implements the Notifier interface for linux
type LinuxNotifier struct{}

// Push sends a notification
func (notifier LinuxNotifier) Push(n Notification) error {
	return exec.Command("notify-send", "-i", n.Icon, n.Title, n.Content).Run()
}
