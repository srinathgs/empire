package heroku

import (
	"encoding/json"
	"net/http"
	"sync"

	"golang.org/x/net/context"

	"github.com/gorilla/mux"
	"github.com/remind101/empire"
	"github.com/remind101/empire/pkg/headerutil"
	"github.com/remind101/empire/pkg/heroku"
	"github.com/remind101/empire/server/auth"
	"github.com/remind101/pkg/reporter"
)

// The Accept header that controls the api version. See
// https://devcenter.heroku.com/articles/platform-api-reference#clients
const AcceptHeader = "application/vnd.heroku+json; version=3"

// API is an http.Handler for the Heroku compatible API of Empire.
type API struct {
	// Authenticator is an auth.Authenticator that will be used to
	// authenticate requests.
	Authenticator auth.Authenticator

	// HandleError is a function that will be called to handle errors
	// returned from handlers.
	handleError func(error, http.ResponseWriter, *http.Request)

	*empire.Empire

	once sync.Once
	mux  *mux.Router
}

// New creates the API routes and returns a new http.Handler to serve them.
func New(e *empire.Empire) *API {
	return &API{
		Empire: e,
		mux:    mux.NewRouter(),
		handleError: func(err error, w http.ResponseWriter, r *http.Request) {
			Error(w, err, http.StatusInternalServerError)
		},
	}
}

// setup sets up all of the routes.
func (a *API) setup() {
	// Apps
	a.handle("/apps", a.GetApps).Methods("GET")
	a.handle("/apps/{app}", a.withApp(a.GetAppInfo)).Methods("GET")
	a.handle("/apps/{app}", a.withApp(a.DeleteApp)).Methods("DELETE")
	a.handle("/apps/{app}", a.withApp(a.PatchApp)).Methods("PATCH")
	a.handle("/apps/{app}/deploys", a.withApp(a.DeployApp)).Methods("POST")
	a.handle("/apps", a.PostApps).Methods("POST")
	a.handle("/organizations/apps", a.PostApps).Methods("POST")

	// Domains
	a.handle("/apps/{app}/domains", a.withApp(a.GetDomains)).Methods("GET")
	a.handle("/apps/{app}/domains", a.withApp(a.PostDomains)).Methods("POST")
	a.handle("/apps/{app}/domains/{hostname}", a.withApp(a.DeleteDomain)).Methods("DELETE")

	// Deploys
	a.handle("/deploys", a.PostDeploys).Methods("POST")

	// Releases
	a.handle("/apps/{app}/releases", a.withApp(a.GetReleases)).Methods("GET")
	a.handle("/apps/{app}/releases/{version}", a.withApp(a.GetRelease)).Methods("GET")
	a.handle("/apps/{app}/releases", a.withApp(a.PostReleases)).Methods("POST")

	// Configs
	a.handle("/apps/{app}/config-vars", a.withApp(a.GetConfigs)).Methods("GET")
	a.handle("/apps/{app}/config-vars", a.withApp(a.PatchConfigs)).Methods("PATCH")

	// Processes
	a.handle("/apps/{app}/dynos", a.withApp(a.GetProcesses)).Methods("GET")
	a.handle("/apps/{app}/dynos", a.withApp(a.PostProcess)).Methods("POST")
	a.handle("/apps/{app}/dynos", a.withApp(a.DeleteProcesses)).Methods("DELETE")
	a.handle("/apps/{app}/dynos/{ptype}.{pid}", a.withApp(a.DeleteProcesses)).Methods("DELETE")
	a.handle("/apps/{app}/dynos/{pid}", a.withApp(a.DeleteProcesses)).Methods("DELETE")

	// Formations
	a.handle("/apps/{app}/formation", a.withApp(a.GetFormation)).Methods("GET")
	a.handle("/apps/{app}/formation", a.withApp(a.PatchFormation)).Methods("PATCH")

	// OAuth
	a.handle("/oauth/authorizations", a.PostAuthorizations).Methods("POST")

	// SSL
	sslRemoved := func(w http.ResponseWriter, r *http.Request) error {
		return ErrSSLRemoved
	}
	a.handle("/apps/{app}/ssl-endpoints", sslRemoved).Methods("GET")
	a.handle("/apps/{app}/ssl-endpoints", sslRemoved).Methods("POST")
	a.handle("/apps/{app}/ssl-endpoints/{cert}", sslRemoved).Methods("PATCH")
	a.handle("/apps/{app}/ssl-endpoints/{cert}", sslRemoved).Methods("DELETE")

	// Logs
	a.handle("/apps/{app}/log-sessions", a.withApp(a.PostLogs)).Methods("POST")
}

// Adds a route. h is wrapped with authentication.
func (a *API) handle(path string, h func(w http.ResponseWriter, r *http.Request) error) *mux.Route {
	return a.mux.Handle(path, a.handler(a.authenticate(h)))
}

func (a *API) withApp(h func(app *empire.App, w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		vars := mux.Vars(r)
		name := vars["app"]

		app, err := a.AppsFind(empire.AppsQuery{Name: &name})
		if err != nil {
			return err
		}
		reporter.AddContext(r.Context(), "app", app.Name)
		return h(app, w, r)
	}
}

// handler returns an http.Handler that calls h. If an error is returned, it
// uses the HandleError method to render the error.
func (a *API) handler(h func(w http.ResponseWriter, r *http.Request) error) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			a.handleError(err, w, r)
			return
		}
	}

	return http.HandlerFunc(handler)
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.once.Do(a.setup)
	a.mux.ServeHTTP(w, r)
}

// Encode json encodes v into w.
func Encode(w http.ResponseWriter, v interface{}) error {
	if v == nil {
		// Empty JSON body "{}"
		v = map[string]interface{}{}
	}

	return json.NewEncoder(w).Encode(v)
}

// Decode json decodes the request body into v.
func Decode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// Stream encodes and flushes data to the client.
func Stream(w http.ResponseWriter, v interface{}) error {
	if err := Encode(w, v); err != nil {
		return err
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	return nil
}

// Error is used to respond with errors in the heroku error format, which is
// specified at
// https://devcenter.heroku.com/articles/platform-api-reference#errors
//
// If an ErrorResource is provided as the error, and it provides a non-zero
// status, that will be used as the response status code.
func Error(w http.ResponseWriter, err error, status int) error {
	res := newError(err)

	// If the ErrorResource provides and exit status, we'll use that
	// instead.
	if res.Status != 0 {
		status = res.Status
	}

	w.WriteHeader(status)
	return Encode(w, res)
}

// NoContent responds with a 404 and an empty body.
func NoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RangeHeader parses the Range header and returns an headerutil.Range.
func RangeHeader(r *http.Request) (headerutil.Range, error) {
	header := r.Header.Get("Range")
	if header == "" {
		return headerutil.Range{}, nil
	}

	rangeHeader, err := headerutil.ParseRange(header)
	if err != nil {
		return headerutil.Range{}, err
	}
	return *rangeHeader, nil
}

// key used to store context values from within this package.
type key int

const (
	userKey key = 0
)

// WithUser adds a user to the context.Context.
func WithUser(ctx context.Context, u *empire.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFromContext returns a user from a context.Context if one is present.
func UserFromContext(ctx context.Context) *empire.User {
	u, ok := ctx.Value(userKey).(*empire.User)
	if !ok {
		panic("expected user to be authenticated")
	}
	return u
}

func findMessage(r *http.Request) (string, error) {
	h := r.Header.Get(heroku.CommitMessageHeader)
	return h, nil
}
