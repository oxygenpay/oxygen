package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleBody struct {
	Message string
}

var sampleBodyValue = sampleBody{Message: "Hello, world!"}

func TestSend(t *testing.T) {
	ctx := context.Background()

	t.Run("Sends webhook", func(t *testing.T) {
		const secret = "my-secret"

		s := assertServer(t, func(t *testing.T, writer http.ResponseWriter, request *http.Request) {
			var body sampleBody
			assertBind(t, request, &body)
			assert.Equal(t, sampleBodyValue, body)

			// Check signature
			signature := request.Header.Get(HeaderSignature)

			assert.NotEmpty(t, signature)
			assert.True(t, ValidateHMAC(lo.Must(json.Marshal(body)), secret, signature))

			writer.WriteHeader(http.StatusOK)
		})

		err := Send(ctx, s.URL, secret, sampleBodyValue)
		assert.NoError(t, err)
	})

	t.Run("Fails", func(t *testing.T) {
		// Not a URL
		err := Send(ctx, "not a url", "", nil)
		assert.ErrorIs(t, err, ErrInvalidInput)

		// Non 2xx response
		s := assertServer(t, func(t *testing.T, writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
		})

		err = Send(ctx, s.URL, "secret", sampleBodyValue)
		assert.ErrorIs(t, err, ErrInvalidStatusCode)
	})
}

func assertBind(t *testing.T, request *http.Request, v any) {
	bytes, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(bytes, v))
}

func assertServer(t *testing.T, handler func(*testing.T, http.ResponseWriter, *http.Request)) *httptest.Server {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		handler(t, writer, request)
	}

	return httptest.NewServer(http.HandlerFunc(fn))
}
