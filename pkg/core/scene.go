package core

// SamplingConfigProvider allows scenes to provide recommended sampling configurations
type SamplingConfigProvider interface {
	RecommendedSamplingConfig() interface{} // Using interface{} to avoid circular import
}
