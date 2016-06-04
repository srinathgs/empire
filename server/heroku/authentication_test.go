package heroku

import (
	"net/http"
	"testing"

	"github.com/remind101/empire"
	"github.com/remind101/pkg/httpx"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
)

type mockAuthenticator struct {
	mock.Mock
}

func (m *mockAuthenticator) Authenticate(username, password, otp string) (*empire.User, error) {
	args := m.Called(username, password, otp)
	user := args.Get(0)
	if user != nil {
		return user.(*empire.User), args.Error(1)
	}
	return nil, args.Error(1)

}

// ensureUserInContext returns and httpx.Handler that raises an error if the
// user isn't set in the context.
func ensureUserInContext(t testing.TB) httpx.Handler {
	return httpx.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		UserFromContext(ctx) // Panics if user is not set.
		return nil
	})
}
