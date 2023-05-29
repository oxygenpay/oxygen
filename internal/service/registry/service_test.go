package registry_test

import (
	"fmt"
	"testing"

	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	t.Run("Get", func(t *testing.T) {
		for _, testCase := range []struct {
			key, value  string
			skipWrite   bool
			expectError bool
		}{
			{key: "abc123", value: "yes"},
			{key: "abc123", value: "yes", skipWrite: true},
			{key: "hello", value: "world", skipWrite: true, expectError: true},
			{key: "my-param", value: "my-value"},
			{key: "my-param", value: "my-value2"},
			{key: "abc123", value: "no"},
			{key: "abc123", value: "no", skipWrite: true},
		} {
			t.Run(fmt.Sprintf("%s/%s", testCase.key, testCase.value), func(t *testing.T) {
				// ARRANGE
				if !testCase.skipWrite {
					_, err := tc.Services.Registry.Set(tc.Context, testCase.key, testCase.value)
					require.NoError(t, err)
				}

				// ACT
				v, err := tc.Services.Registry.Get(tc.Context, testCase.key)

				// ASSERT
				if testCase.expectError {
					assert.Error(t, err)
					return
				}

				assert.Equal(t, testCase.key, v.Key)
				assert.Equal(t, testCase.value, v.Value)
			})
		}
	})

	t.Run("GetBoolSafe", func(t *testing.T) {
		for _, testCase := range []struct {
			key, value   string
			defaultValue bool
			skipWrite    bool
			expected     bool
		}{
			{key: "bool", value: "false", defaultValue: false, expected: false},
			{key: "bool", value: "false", defaultValue: true, expected: false},
			{key: "bool", value: "0", defaultValue: true, expected: false},
			{key: "bool", value: "1", defaultValue: false, expected: true},
			{key: "bool", skipWrite: true, defaultValue: true, expected: true},
		} {
			t.Run(fmt.Sprintf("%s/%s", testCase.key, testCase.value), func(t *testing.T) {
				// ARRANGE
				if !testCase.skipWrite {
					_, err := tc.Services.Registry.Set(tc.Context, testCase.key, testCase.value)
					require.NoError(t, err)
				}

				// ACT
				v := tc.Services.Registry.GetBoolSafe(tc.Context, testCase.key, false)

				// ASSERT
				assert.Equal(t, testCase.expected, v)
			})
		}
	})

	t.Run("GetStringsSafe", func(t *testing.T) {
		for _, testCase := range []struct {
			key, value string
			skipWrite  bool
			expected   []string
		}{
			{key: "list_a", value: "a,b, c", expected: []string{"a", "b", "c"}},
			{key: "list_b", value: "", expected: nil},
			{key: "list_c", skipWrite: true, expected: nil},
			{key: "list_d", value: "test@me.com", expected: []string{"test@me.com"}},
			{key: "list_e", value: "test@me.com,test2@me.com", expected: []string{"test@me.com", "test2@me.com"}},
		} {
			t.Run(fmt.Sprintf("%s/%s", testCase.key, testCase.value), func(t *testing.T) {
				// ARRANGE
				if !testCase.skipWrite {
					_, err := tc.Services.Registry.Set(tc.Context, testCase.key, testCase.value)
					require.NoError(t, err)
				}

				// ACT
				v := tc.Services.Registry.GetStringsSafe(tc.Context, testCase.key)

				// ASSERT
				assert.Equal(t, testCase.expected, v)
			})
		}
	})
}
