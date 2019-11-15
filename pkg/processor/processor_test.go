package processor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oleg-balunenko/spamassassin-parser/pkg/models"
	"github.com/oleg-balunenko/spamassassin-parser/pkg/utils"
)

func TestProcessReports(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	defer cancel()

	cfg := NewConfig()
	cfg.Receive.Errors = true

	processor := NewProcessor(cfg)

	go processor.Process(ctx)

	type want struct {
		filepath string
		wantErr  bool
	}

	type input struct {
		filepath string
		testID   string
	}

	var tests = []struct {
		input input
		want  want
	}{
		{
			input: input{
				filepath: filepath.Join("..", "testdata", "report1.txt"),
				testID:   "report1.txt",
			},
			want: want{
				filepath: filepath.Join("..", "testdata", "report1.golden.json"),
				wantErr:  false,
			},
		},
		{
			input: input{
				filepath: filepath.Join("..", "testdata", "report2.txt"),
				testID:   "report2.txt",
			},
			want: want{
				filepath: filepath.Join("..", "testdata", "report2.golden.json"),
				wantErr:  false,
			},
		},
		{
			input: input{
				filepath: filepath.Join("..", "testdata", "report1.txt"),
				testID:   "report1.txt.repeat",
			},
			want: want{
				filepath: filepath.Join("..", "testdata", "report1.golden.json"),
				wantErr:  false,
			},
		},
		{
			input: input{
				filepath: filepath.Join("..", "testdata", "empty.json"),
				testID:   "empty",
			},
			want: want{
				filepath: filepath.Join("..", "testdata", "empty.json"),
				wantErr:  true,
			},
		},
	}

	type expected struct {
		report  models.Report
		wantErr bool
	}

	expResults := make(map[string]expected, len(tests))

	for _, tt := range tests {
		tt := tt
		report := utils.GetReportFromFile(t, tt.want.filepath)

		expResults[tt.input.testID] = expected{
			report:  report,
			wantErr: tt.want.wantErr,
		}
	}

	go func() {
		for _, tt := range tests {
			tt := tt
			file := utils.GetReaderFromFile(t, tt.input.filepath)
			t.Logf("processing report: %s \n", tt.input.testID)
			processor.Input() <- &models.ProcessorInput{
				Data:   file,
				TestID: tt.input.testID,
			}
		}

		processor.Close()
	}()

	// check all reports processed
	var processed int
LOOP:
	for {
		select {
		case res := <-processor.Results():
			if res != nil {
				processed++
				t.Logf("received result: %s\n", res.TestID)
				exp := expResults[res.TestID]

				assert.Equal(t, exp.report, res.Report)
			}
		case err := <-processor.Errors():
			require.IsType(t, &models.Error{}, err, "unexpected error type")
			merr := err.(*models.Error)
			exp := expResults[merr.TestID]

			if exp.wantErr {
				assert.Error(t, err)
				processed++
				continue
			}
			assert.NoError(t, err)

		case <-ctx.Done():
			assert.Equal(t, len(expResults), processed, "deadline reached, but not all results received")
			time.Sleep(time.Second * 2)
			break LOOP
		}
	}
}

func TestNewConfig(t *testing.T) {
	expConfgig := &Config{
		Buffer: 0,
		Receive: struct {
			Response bool
			Errors   bool
		}{
			Response: true,
			Errors:   false,
		},
	}
	got := NewConfig()
	require.Equal(t, expConfgig, got)
}

func TestNewDefaultProcessor(t *testing.T) {
	got := NewDefaultProcessor()
	assert.NotNil(t, got)
	assert.IsType(t, &processor{}, got)
	assert.NotNil(t, got.Results())
	assert.Nil(t, got.Errors())
	assert.NotNil(t, got.Input())
}
