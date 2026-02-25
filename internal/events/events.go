// package events contains message types shared between web and tui packages.
package events

// WebSessionActionMsg is sent by the web server after session mutations.
type WebSessionActionMsg struct {
	ContainerID string
}

// WebListenURLMsg is sent when the web server starts listening.
type WebListenURLMsg struct{ URL string }

// TailscaleURLMsg is sent when the tailscale FQDN becomes available.
type TailscaleURLMsg struct{ URL string }
