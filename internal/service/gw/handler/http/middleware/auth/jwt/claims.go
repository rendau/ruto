package jwt

func getNumericClaim(v any) (float64, bool, bool) {
	if v == nil {
		return 0, false, true
	}

	switch value := v.(type) {
	case float64:
		return value, true, true
	case int64:
		return float64(value), true, true
	case int:
		return float64(value), true, true
	default:
		return 0, true, false
	}
}
