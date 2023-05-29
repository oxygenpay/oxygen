package merchantapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

func (h *Handler) CreateFormSubmission(c echo.Context) error {
	var req model.FormRequest
	if !common.BindAndValidateRequest(c, &req) {
		return nil
	}

	u := middleware.ResolveUser(c)
	if u == nil {
		return errors.New("unable to resolve user")
	}

	event := bus.FormSubmittedEvent{
		RequestType: *req.Topic,
		Message:     *req.Message,
		UserID:      u.ID,
	}

	if err := h.publisher.Publish(bus.TopicFormSubmissions, event); err != nil {
		return errors.Wrap(err, "unable to publish event")
	}

	return c.NoContent(http.StatusCreated)
}
