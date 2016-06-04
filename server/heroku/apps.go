package heroku

import (
	"net/http"

	"github.com/remind101/empire"
	"github.com/remind101/empire/pkg/heroku"
)

type App heroku.App

func newApp(a *empire.App) *App {
	return &App{
		Id:        a.ID,
		Name:      a.Name,
		CreatedAt: *a.CreatedAt,
		Cert:      a.Cert,
	}
}

func newApps(as []*empire.App) []*App {
	apps := make([]*App, len(as))

	for i := 0; i < len(as); i++ {
		apps[i] = newApp(as[i])
	}

	return apps
}

func (h *API) GetApps(w http.ResponseWriter, r *http.Request) error {
	apps, err := h.Apps(empire.AppsQuery{})
	if err != nil {
		return err
	}

	w.WriteHeader(200)
	return Encode(w, newApps(apps))
}

func (h *API) GetAppInfo(a *empire.App, w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(200)
	return Encode(w, newApp(a))
}

func (h *API) DeleteApp(a *empire.App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	m, err := findMessage(r)
	if err != nil {
		return err
	}

	if err := h.Destroy(ctx, empire.DestroyOpts{
		User:    UserFromContext(ctx),
		App:     a,
		Message: m,
	}); err != nil {
		return err
	}

	return NoContent(w)
}

func (h *API) DeployApp(a *empire.App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	opts, err := newDeploymentsCreateOpts(ctx, w, r)
	opts.App = a
	if err != nil {
		return err
	}
	h.Deploy(ctx, *opts)
	return nil
}

type PostAppsForm struct {
	Name string `json:"name"`
}

func (h *API) PostApps(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var form PostAppsForm

	if err := Decode(r, &form); err != nil {
		return err
	}

	m, err := findMessage(r)
	if err != nil {
		return err
	}

	a, err := h.Create(ctx, empire.CreateOpts{
		User:    UserFromContext(ctx),
		Name:    form.Name,
		Message: m,
	})
	if err != nil {
		return err
	}

	w.WriteHeader(201)
	return Encode(w, newApp(a))
}

func (h *API) PatchApp(a *empire.App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var form heroku.AppUpdateOpts

	if err := Decode(r, &form); err != nil {
		return err
	}

	if form.Cert != nil {
		if err := h.CertsAttach(ctx, a, *form.Cert); err != nil {
			return err
		}
	}

	return Encode(w, newApp(a))
}
