package userevents_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/event/userevents"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*test.IntegrationTest, *userevents.Handler, *[]string) {
	tc := test.NewIntegrationTest(t)

	var responses []string

	okResponder := func(writer http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		responses = append(responses, string(raw))

		writer.WriteHeader(http.StatusOK)
	}

	handler := userevents.New(
		"local",
		httptest.NewServer(http.HandlerFunc(okResponder)).URL,
		tc.Services.Users,
		tc.Logger,
	)

	return tc, handler, &responses
}

func TestHandler_UserRegistered(t *testing.T) {
	tc, handler, responses := setup(t)

	// ARRANGE
	// Given a user
	u, _ := tc.Must.CreateUser(t, auth.GoogleUser{
		Name:          "Bob",
		Email:         "bob@gmail.com",
		EmailVerified: true,
	})

	// ACT
	err := handler.UserRegistered(tc.Context, marshal(bus.UserRegisteredEvent{UserID: u.ID}))

	// ASSERT
	require.NoError(t, err)
	assert.Len(t, *responses, 1)
	assert.Contains(t, (*responses)[0], "local")
	assert.Contains(t, (*responses)[0], "Bob")
	assert.Contains(t, (*responses)[0], "bob@gmail.com")
}

func TestHandler_FormSubmitted(t *testing.T) {
	tc, handler, responses := setup(t)

	// ARRANGE
	// Given a user
	u, _ := tc.Must.CreateUser(t, auth.GoogleUser{
		Name:          "Bob",
		Email:         "bob@gmail.com",
		EmailVerified: true,
	})

	event := bus.FormSubmittedEvent{
		RequestType: "support",
		Message:     "Hey! Do you support USDC?",
		UserID:      u.ID,
	}

	// ACT
	err := handler.FormSubmitted(tc.Context, marshal(event))

	// ASSERT
	require.NoError(t, err)
	assert.Len(t, *responses, 1)
	assert.Contains(t, (*responses)[0], "local")
	assert.Contains(t, (*responses)[0], "bob@gmail.com")
	assert.Contains(t, (*responses)[0], event.RequestType)
	assert.Contains(t, (*responses)[0], event.Message)
}

func marshal(v any) []byte {
	return lo.Must(json.Marshal(v))
}
