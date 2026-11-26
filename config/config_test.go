package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/maadiii/hexa/config"
)

func TestEnvReportsEveryProblem(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"PORT": "eighty", "MODE": "fast", "NAME": " "}
	env := config.New(func(key string) (string, bool) {
		v, ok := vars[key]

		return v, ok
	})

	port := env.Int("PORT", 8080)
	env.OneOf("MODE", "safe", "safe", "strict")
	env.Required("NAME")
	timeout := env.Duration("TIMEOUT", 5*time.Second)

	if port != 8080 || timeout != 5*time.Second {
		t.Errorf("got port %d and timeout %s, want the defaults", port, timeout)
	}

	err := env.Err()
	if err == nil {
		t.Fatal("want an error, got nil")
	}

	for _, key := range []string{"PORT", "MODE", "NAME"} {
		if !strings.Contains(err.Error(), key+":") {
			t.Errorf("errors doesn't mention %s\n%v", key, err)
		}
	}
}
