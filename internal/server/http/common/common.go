package common

import (
	"fmt"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
)

const StatusInternalError = "internal_error"

func BindAndValidateRequest(c echo.Context, req Validatable) bool {
	if err := c.Bind(req); err != nil {
		_ = c.JSON(http.StatusBadRequest, &model.ErrorResponse{
			Errors:  []*model.ErrorResponseItem{{Message: "Invalid JSON"}},
			Message: "Bad request",
			Status:  "validation_error",
		})
		return false
	}

	if err := req.Validate(strfmt.Default); err != nil {
		_ = ValidationErrorResponse(c, err)
		return false
	}

	return true
}

type Validatable interface {
	Validate(formats strfmt.Registry) error
}

func ValidationErrorResponse(c echo.Context, reason any) error {
	var responseErrors []*model.ErrorResponseItem

	if errStr, ok := reason.(string); ok {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
			Message: "Bad request",
			Status:  "validation_error",
			Errors:  []*model.ErrorResponseItem{{Message: errStr}},
		})
	}

	err, ok := reason.(error)
	if !ok {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
			Message: "Bad request",
			Status:  "validation_error",
		})
	}

	// try to cast errors
	compositeError, ok := err.(*errors.CompositeError)
	if ok {
		for _, fieldErrorRaw := range compositeError.Errors {
			if fieldError, ok := fieldErrorRaw.(*errors.Validation); ok {
				responseErrors = append(responseErrors, &model.ErrorResponseItem{
					Field:   fieldError.Name,
					Message: fieldError.Error(),
				})
			}

			if fieldError, ok := fieldErrorRaw.(*errorWrapper); ok {
				responseErrors = append(responseErrors, fieldError.item)
			}
		}
	} else {
		responseErrors = append(responseErrors, &model.ErrorResponseItem{
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
		Message: "Bad request",
		Status:  "validation_error",
		Errors:  responseErrors,
	})
}

func ValidationErrorItemResponse(c echo.Context, field, message string, args ...any) error {
	return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
		Message: "Bad request",
		Status:  "validation_error",
		Errors: []*model.ErrorResponseItem{
			{Field: field, Message: fmt.Sprintf(message, args...)},
		},
	})
}

func NotFoundResponse(c echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
		Message: message,
		Status:  "not_found",
		Errors:  nil,
	})
}

// UUID parsed uuid from request's path or query. If failed, writes validation error response.
func UUID(c echo.Context, param string) (uuid.UUID, error) {
	value := c.Param(param)
	if value == "" {
		value = c.QueryParam(param)
	}

	id, err := uuid.Parse(value)
	if err != nil {
		errMsg := fmt.Errorf("invalid UUID '%s'", param)

		if vErr := ValidationErrorResponse(c, errMsg); vErr != nil {
			return uuid.Nil, vErr
		}

		return uuid.Nil, errMsg
	}

	return id, nil
}
