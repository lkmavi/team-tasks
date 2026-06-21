package slogx

// RemovalSet is a collection of attribute keys to be excluded from log output.
type RemovalSet struct {
	keys []string
}

// NewRemovalSet creates a RemovalSet optionally pre-populated with keys.
func NewRemovalSet(keys ...string) *RemovalSet {
	return &RemovalSet{keys: keys}
}

// Add appends one or more keys to the removal set.
func (s *RemovalSet) Add(keys ...string) *RemovalSet {
	s.keys = append(s.keys, keys...)
	return s
}

// Keys returns the underlying slice of keys.
func (s *RemovalSet) Keys() []string {
	return s.keys
}
