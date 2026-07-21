package ports

// Notifier delivers a transient native desktop notification. The title and
// body must already be scrubbed and capped by the caller (use case layer).
// Implementations may silently no-op on unsupported platforms.
type Notifier interface {
	Notify(title, body string) error
}
