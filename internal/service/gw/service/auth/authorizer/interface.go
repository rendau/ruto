package authorizer

import (
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Authorizer interface {
	Authorize(req *model.AuthRequest) bool
}
