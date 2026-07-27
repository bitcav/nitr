package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.Equal(t, "0.8.1", Version)
}
