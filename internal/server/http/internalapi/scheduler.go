package internalapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/scheduler"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-admin/v1/model"
)

func (h *Handler) RunSchedulerJob(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.RunJobRequest
	if !common.BindAndValidateRequest(c, &req) {
		return nil
	}

	allJobs := []string{
		"performInternalWalletTransfer",
		"checkInternalTransferProgress",
		"performWithdrawalsCreation",
		"checkWithdrawalsProgress",
		"cancelExpiredPayments",
	}

	jobID := fmt.Sprintf("%s-web-%d", req.Job, time.Now().UTC().Unix())
	ctx = context.WithValue(ctx, scheduler.ContextJobID{}, jobID)
	ctx = h.logger.WithContext(ctx)

	var errJob error
	switch req.Job {
	case "checkIncomingTransactionsProgress":
		errJob = h.scheduler.CheckIncomingTransactionsProgress(ctx)
	case "performInternalWalletTransfer":
		errJob = h.scheduler.PerformInternalWalletTransfer(ctx)
	case "checkInternalTransferProgress":
		errJob = h.scheduler.CheckInternalTransferProgress(ctx)
	case "performWithdrawalsCreation":
		errJob = h.scheduler.PerformWithdrawalsCreation(ctx)
	case "checkWithdrawalsProgress":
		errJob = h.scheduler.CheckWithdrawalsProgress(ctx)
	case "cancelExpiredPayments":
		errJob = h.scheduler.CancelExpiredPayments(ctx)
	default:
		return common.ValidationErrorResponse(c, fmt.Sprintf(
			"job %s not found. Available jobs: %s",
			req.Job,
			strings.Join(allJobs, ", "),
		))
	}

	logs, err := h.scheduler.JobLogger().ListByJobID(ctx, jobID, 1000)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	errorMessage := ""
	if errJob != nil {
		errorMessage = errJob.Error()
	}

	return c.JSON(http.StatusOK, &model.JobResults{
		Error: errorMessage,
		Logs: util.MapSlice(logs, func(l *log.JobLog) *model.Log {
			return &model.Log{
				Message:  l.Message,
				Metadata: l.Metadata,
			}
		}),
	})
}
