package v1

const (
	WebhookServiceName = "multiclusterhub-operator-webhook"
)

// AvailabilityConfigIsValid ...
func AvailabilityConfigIsValid(config AvailabilityType) bool {
	switch config {
	case HAHigh, HABasic:
		return true
	default:
		return false
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
