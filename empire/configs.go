package empire

import (
	"database/sql"
	"database/sql/driver"

	"github.com/lib/pq/hstore"
)

// Config represents a collection of environment variables.
type Config struct {
	ID      string `json:"id" db:"id"`
	Vars    Vars   `json:"vars" db:"vars"`
	AppName string `json:"-" db:"app_id"`
}

// NewConfig initializes a new config based on the old config, with the new
// variables provided.
func NewConfig(old *Config, vars Vars) *Config {
	v := mergeVars(old.Vars, vars)

	return &Config{
		AppName: old.AppName,
		Vars:    v,
	}
}

// Variable represents the name of an environment variable.
type Variable string

// Vars represents a variable -> value mapping.
type Vars map[Variable]string

// Scan implements the sql.Scanner interface.
func (v *Vars) Scan(src interface{}) error {
	h := hstore.Hstore{}
	if err := h.Scan(src); err != nil {
		return err
	}

	vars := make(Vars)

	for k, v := range h.Map {
		vars[Variable(k)] = v.String
	}

	*v = vars

	return nil
}

// Value implements the driver.Value interface.
func (v Vars) Value() (driver.Value, error) {
	m := make(map[string]sql.NullString)

	for k, v := range v {
		m[string(k)] = sql.NullString{
			Valid:  true,
			String: v,
		}
	}

	h := hstore.Hstore{
		Map: m,
	}

	return h.Value()
}

type ConfigsCreator interface {
	ConfigsCreate(*Config) (*Config, error)
}

type ConfigsFinder interface {
	ConfigsFind(id string) (*Config, error)
	ConfigsCurrent(*App) (*Config, error)
}

type ConfigsApplier interface {
	ConfigsApply(*App, Vars) (*Config, error)
}

type ConfigsService interface {
	ConfigsCreator
	ConfigsFinder
	ConfigsApplier
}

type configsService struct {
	*db
}

func (s *configsService) ConfigsCreate(config *Config) (*Config, error) {
	return ConfigsCreate(s.db, config)
}

func (s *configsService) ConfigsCurrent(app *App) (*Config, error) {
	return ConfigsCurrent(s.db, app)
}

func (s *configsService) ConfigsFind(id string) (*Config, error) {
	return ConfigsFind(s.db, id)
}

func (s *configsService) ConfigsApply(app *App, vars Vars) (*Config, error) {
	return ConfigsApply(s.db, app, vars)
}

// ConfigsCreate inserts a Config in the database.
func ConfigsCreate(db *db, config *Config) (*Config, error) {
	return config, db.Insert(config)
}

// ConfigsCurrent returns the current Config for the given app, creating it if
// it does not already exist.
func ConfigsCurrent(db *db, app *App) (*Config, error) {
	c, err := ConfigsFindByApp(db, app)
	if err != nil {
		return nil, err
	}

	if c != nil {
		return c, nil
	}

	return ConfigsCreate(db, &Config{
		AppName: app.Name,
		Vars:    make(Vars),
	})
}

// ConfigsFind finds a Config by id.
func ConfigsFind(db *db, id string) (*Config, error) {
	return ConfigsFindBy(db, "id", id)
}

// ConfigsFindByApp finds the current config for the given App.
func ConfigsFindByApp(db *db, app *App) (*Config, error) {
	return ConfigsFindBy(db, "app_id", app.Name)
}

// ConfigsApply gets the current config for the given app, copies it, merges the
// new Vars in, then inserts it.
func ConfigsApply(db *db, app *App, vars Vars) (*Config, error) {
	c, err := ConfigsCurrent(db, app)
	if err != nil {
		return nil, err
	}

	// If the app doesn't have a config, just build a new one.
	if c == nil {
		c = &Config{
			AppName: app.Name,
		}
	}

	return ConfigsCreate(db, NewConfig(c, vars))
}

// ConfigsFindBy finds a Config by a field.
func ConfigsFindBy(db *db, field string, value interface{}) (*Config, error) {
	var config Config

	if err := db.SelectOne(&config, `select id, app_id, vars from configs where `+field+` = $1 order by created_at desc limit 1`, value); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	return &config, nil
}

// mergeVars copies all of the vars from a, and merges b into them, returning a
// new Vars.
func mergeVars(old, new Vars) Vars {
	vars := make(Vars)

	for n, v := range old {
		vars[n] = v
	}

	for n, v := range new {
		if v != "" {
			vars[n] = v
		} else {
			delete(vars, n)
		}
	}

	return vars
}
