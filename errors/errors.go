package errors

import "errors"

var (
	ErrNoActiveAlgorithm = errors.New("No active algorithm configured")
	ErrNoPolicyFound     = errors.New("No policy found for api key")
)
