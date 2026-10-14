package changerequest

import (
	"errors"

	"github.com/rendau/ruto/internal/errs"
)

func invalid(desc string) error {
	return errs.ErrFull{Err: errs.InvalidRequest, Desc: desc}
}

func outdated(desc string) error {
	return errs.ErrFull{Err: errs.ChangeRequestOutdated, Desc: desc}
}

func errNoPermission() error {
	return errs.NoPermission
}

func errNotFound(kind, id string) error {
	return errs.ErrFull{Err: errs.ObjectNotFound, Desc: kind + " " + id + " not found"}
}

// errsBase returns the sentinel code of err, if it carries one.
func errsBase(err error) (error, bool) {
	if full, ok := errors.AsType[errs.ErrFull](err); ok {
		return full.Err, true
	}
	if base, ok := errors.AsType[errs.Err](err); ok {
		return base, true
	}
	return nil, false
}

// errDesc is the human part of err: the description of ErrFull, otherwise
// the whole text.
func errDesc(err error) string {
	if full, ok := errors.AsType[errs.ErrFull](err); ok && full.Desc != "" {
		return full.Desc
	}
	return err.Error()
}
