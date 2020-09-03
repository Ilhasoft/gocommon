package storage_test

import (
	"os"
	"testing"

	"github.com/nyaruka/gocommon/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFS(t *testing.T) {
	s := storage.NewFS("_testing")
	assert.NoError(t, s.Test())

	// break our ability to write to that directory
	require.NoError(t, os.Chmod("_testing", 0555))

	assert.EqualError(t, s.Test(), "open _testing/test.txt: permission denied")

	require.NoError(t, os.Chmod("_testing", 0777))

	url, err := s.Put("/foo/bar.txt", "text/plain", []byte(`hello world`))
	assert.NoError(t, err)
	assert.Equal(t, "_testing/foo/bar.txt", url)

	_, data, err := s.Get("/foo/bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, []byte(`hello world`), data)

	require.NoError(t, os.RemoveAll("_testing"))
}
