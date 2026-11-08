package api

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Huma echoes the offending input of a validation failure back in errors[].value. Request
// bodies carry passwords and other secrets, so body values are never echoed; the message and
// location still say what was wrong.
func init() {
	base := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		err := base(status, msg, errs...)
		if model, ok := err.(*huma.ErrorModel); ok {
			for _, detail := range model.Errors {
				if detail != nil && strings.HasPrefix(detail.Location, "body") {
					detail.Value = nil
				}
			}
		}
		return err
	}
}
