package core

// Logger interface for raytracer logging
type Logger interface {
	Printf(format string, args ...interface{})
}
