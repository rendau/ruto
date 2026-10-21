package mcp

import (
	"errors"

	"github.com/rendau/ruto/internal/errs"
)

var (
	errKeyMismatch = errors.New("the api key or token of this call differs from the one the MCP session was opened with")
	errKeyInvalid  = errors.New("the api key or token is not valid anymore")
)

// toolErr renders an error for the agent: the code and description of ruto
// errors, the text otherwise.
func toolErr(err error) error {
	if full, ok := errors.AsType[errs.ErrFull](err); ok {
		msg := full.Err.Error()
		if full.Desc != "" {
			msg += ": " + full.Desc
		}
		for field, desc := range full.Fields {
			msg += "; " + field + ": " + desc
		}
		return errors.New(msg)
	}
	return err
}
