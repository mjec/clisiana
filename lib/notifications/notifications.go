package notifications

import (
	"runtime"
)

// Notifier is the interface implemented for various operating systems
type Notifier interface {
	Push(Notification) error
}

// Notification is a structure for desktop notifications to be displayed.
// Only Title and Content are supported on all platforms.
type Notification struct {
	Title   string
	Content string
	Icon    string
}

// OSAppropriateNotifier returns a notifier appropriate for the currently running operating system
// This will *always* return, including by returning a dummy notifier if nothing else matches
func OSAppropriateNotifier() Notifier {
	switch runtime.GOOS {
	case "darwin":
		return OSXNotifier{}
	case "linux":
		return LinuxNotifier{}
	case "dragonfly":
		break // FIXME: Not implemented
	case "freebsd":
		break // FIXME: Not implemented
	case "netbsd":
		break // FIXME: Not implemented
	case "openbsd":
		break // FIXME: Not implemented
	case "plan9":
		break // FIXME: Not implemented
	case "solaris":
		break // FIXME: Not implemented
	case "windows":
		break // FIXME: Not implemented
	}
	return dummyNotifier{}
}

// DummyNotifier returns a dummy notifier which has no effects
func DummyNotifier() Notifier {
	return dummyNotifier{}
}

// dummyNotifier implements the Notifier interface without any effects
type dummyNotifier struct{}

// Push sends a notification
func (notifier dummyNotifier) Push(n Notification) error {
	return nil
}
