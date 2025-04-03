package collector

import (
	"bytes"

	"sync"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/common/promslog"
	_ "github.com/sijms/go-ora/v2"
	"github.com/stretchr/testify/assert"
)

func TestMalformedDSNMasksUserPassword(t *testing.T) {
	buf := bytes.Buffer{}
	w := log.NewSyncWriter(&buf)
	e := &Exporter{
		mu:     &sync.Mutex{},
		dsn:    "\tuser:pass@sdfoijwef/sdfle",
		logger: promslog.New(&promslog.Config{Writer: w}),
	}
	err := e.connect()
	assert.NotNil(t, err)
	assert.Contains(t, buf.String(), "malformed DSN\" value=***@")
}
