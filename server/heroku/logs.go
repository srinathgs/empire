package heroku

import (
	"net/http"
	"time"

	"github.com/remind101/empire"
	streamhttp "github.com/remind101/empire/pkg/stream/http"
)

type PostLogsForm struct {
	Duration int64
}

func (h *API) PostLogs(a *empire.App, w http.ResponseWriter, r *http.Request) error {
	var form PostLogsForm

	if err := Decode(r, &form); err != nil {
		return err
	}

	rw := streamhttp.StreamingResponseWriter(w)

	// Prevent the ELB idle connection timeout to close the connection.
	defer close(streamhttp.Heartbeat(rw, 10*time.Second))

	err := h.StreamLogs(a, rw, time.Duration(form.Duration))
	if err != nil {
		return err
	}

	return nil
}
