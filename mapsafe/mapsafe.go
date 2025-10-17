package mapsafe

// Get retrieves a typed value from a map[string]any.
// If the key is missing or the type cannot be converted, it returns the default value.
func Get[T any](m map[string]any, key string, defaultValue T) T {
	if val, ok := m[key]; ok {
		switch any(defaultValue).(type) {
		case int:
			switch x := val.(type) {
			case int:
				return any(x).(T)
			case float64:
				return any(int(x)).(T)
			}
		case float64:
			switch x := val.(type) {
			case float64:
				return any(x).(T)
			case int:
				return any(float64(x)).(T)
			}
		case string:
			if s, ok := val.(string); ok {
				return any(s).(T)
			}
		case bool:
			if b, ok := val.(bool); ok {
				return any(b).(T)
			}
		default:
			// fallback: if type matches exactly
			if v2, ok := val.(T); ok {
				return v2
			}
		}
	}
	return defaultValue
}
