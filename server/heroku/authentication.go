package heroku

import (
	"net/http"

	"github.com/remind101/empire/server/auth"
	"github.com/remind101/pkg/logger"
	"github.com/remind101/pkg/reporter"
)

func (a *API) authenticate(h func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		username, password, ok := r.BasicAuth()
		if !ok {
			return ErrUnauthorized
		}

		user, err := a.Authenticator.Authenticate(username, password, r.Header.Get(HeaderTwoFactor))
		if err != nil {
			switch err {
			case auth.ErrTwoFactor:
				return ErrTwoFactor
			case auth.ErrForbidden:
				return ErrUnauthorized
			}

			if err, ok := err.(*auth.UnauthorizedError); ok {
				return errUnauthorized(err)
			}

			return &ErrorResource{
				Status:  http.StatusForbidden,
				ID:      "forbidden",
				Message: err.Error(),
			}
		}

		// Embed the associated user into the context.
		r = r.WithContext(WithUser(r.Context(), user))

		logger.Info(r.Context(),
			"authenticated",
			"user", user.Name,
		)

		reporter.AddContext(r.Context(), "user", user.Name)

		return h(w, r)
	}
}
