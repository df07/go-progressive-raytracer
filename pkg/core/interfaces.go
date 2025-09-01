package core

// Preprocessor interface for objects that need scene preprocessing
// Note: Most preprocessors now take parameters directly rather than full BVH

// Scene interface for scene management
// Scene interface removed - use scene.Scene struct directly

// Logger interface for raytracer logging
type Logger interface {
	Printf(format string, args ...interface{})
}
