//nolint:testpackage // These tests will either be removed or converted to Gingko.
package helpers

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("test_env", "test_val")
	defer os.Unsetenv("test_env")

	cases := []struct {
		name          string
		envKey        string
		defaultValue  string
		expectedValue string
	}{
		{
			name:          "env exists",
			envKey:        "test_env",
			expectedValue: "test_val",
		},
		{
			name:          "env does not exist",
			envKey:        "nonexistent",
			defaultValue:  "default_val",
			expectedValue: "default_val",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value := GetEnv(c.envKey, c.defaultValue)
			if value != c.expectedValue {
				t.Errorf("expect %v, but got: %v", c.expectedValue, value)
			}
		})
	}
}
