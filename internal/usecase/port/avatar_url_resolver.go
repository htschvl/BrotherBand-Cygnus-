package port

// AvatarURLResolver translates a stored media key into a public CDN
// URL. Use cases that surface user-facing avatars depend on this
// narrow port instead of the full `media.ImageStore`, so their mock
// surface stays minimal (ISP).
//
// The R2 presigner — which implements media.ImageStore — also
// satisfies this interface for free.
type AvatarURLResolver interface {
	PublicURL(mediaKey string) string
}
