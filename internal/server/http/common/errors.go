package common

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
)

type errorWrapper struct {
	item *model.ErrorResponseItem
}

func (w *errorWrapper) Error() string {
	return fmt.Sprintf("%s: %s", w.item.Field, w.item.Message)
}

func WrapErrorItem(item *model.ErrorResponseItem) error {
	return &errorWrapper{item}
}

//nolint:unparam
func ErrorResponse(c echo.Context, status string) error {
	return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{
		Message: "Server error",
		Status:  status,
	})
}
