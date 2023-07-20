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

	jobID := fmt.Sprintf("%s-web-%d", req.Job, time.Now().UTC().Unix())
	ctx = context.WithValue(ctx, scheduler.ContextJobID{}, jobID)
	ctx = h.logger.WithContext(ctx)

	jobs := map[string]func(context.Context) error{
		"checkIncomingTransactionsProgress": h.scheduler.CheckIncomingTransactionsProgress,
		"performInternalWalletTransfer":     h.scheduler.PerformInternalWalletTransfer,
		"checkInternalTransferProgress":     h.scheduler.CheckInternalTransferProgress,
		"performWithdrawalsCreation":        h.scheduler.PerformWithdrawalsCreation,
		"checkWithdrawalsProgress":          h.scheduler.CheckWithdrawalsProgress,
		"cancelExpiredPayments":             h.scheduler.CancelExpiredPayments,
		"ensureOutboundWallets":             h.scheduler.EnsureOutboundWallets,
	}

	job, exists := jobs[req.Job]
	if !exists {
		return common.ValidationErrorResponse(c, fmt.Sprintf(
			"job %s not found. Available jobs: %s",
			req.Job,
			strings.Join(util.Keys(jobs), ", "),
		))
	}

	errJob := job(ctx)

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
