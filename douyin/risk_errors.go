package douyin

import "errors"

var (
	// ErrRiskBdturing identifies a bdturing human-verification challenge.
	ErrRiskBdturing = errors.New("bdturing risk challenge")
	// ErrRiskPassport identifies a passport secondary-verification challenge.
	ErrRiskPassport = errors.New("passport risk verification")
	// ErrRiskEmptyResponse identifies an empty anti-bot response.
	ErrRiskEmptyResponse = errors.New("empty risk response")
	// ErrRiskACrawler identifies an acrawler challenge page.
	ErrRiskACrawler = errors.New("acrawler risk challenge")
	// ErrRiskNonJSON identifies a non-JSON response from a JSON endpoint.
	ErrRiskNonJSON = errors.New("non-JSON risk response")
)

// RiskError preserves the readable risk message and exposes a stable category.
type RiskError struct {
	Reason     error
	StatusCode int
	LogID      string
	Detail     string
	Message    string
}

func (e *RiskError) Error() string {
	return e.Message
}

func (e *RiskError) Unwrap() error {
	return e.Reason
}
