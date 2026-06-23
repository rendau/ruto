package model

// RedactedPlaceholder is the value substituted for secret data when a model is
// returned to a user that may view but not manage the owning app.
const RedactedPlaceholder = "••••••"

// RedactedValues returns a copy of vars with the same keys but every value
// replaced by RedactedPlaceholder. The receiver is left untouched.
func RedactedValues(vars Vars) Vars {
	if len(vars) == 0 {
		return vars
	}

	result := New(len(vars))
	for k := range vars {
		result[k] = RedactedPlaceholder
	}

	return result
}
