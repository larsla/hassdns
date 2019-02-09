package monitoring

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/errorreporting"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
)

var (
	logger    = log.New(os.Stdout, "", 0)
	errLogger = log.New(os.Stderr, "", 0)
)

type ErrorReporter struct {
	errorClient *errorreporting.Client
}

func NewErrorReporter(name string) (*ErrorReporter, error) {
	ctx := context.Background()

	credentials, err := google.FindDefaultCredentials(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default credentials")
	}

	fmt.Printf("Current project: %s\n", credentials.ProjectID)

	errorClient, err := errorreporting.NewClient(ctx, credentials.ProjectID, errorreporting.Config{
		ServiceName: name,
		OnError: func(err error) {
			errLogger.Printf("Could not log error: %v", err)
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup error reporting client")
	}

	return &ErrorReporter{errorClient}, nil
}

func (e *ErrorReporter) Close() error {
	return e.errorClient.Close()
}

func (e *ErrorReporter) Log(err error) {
	e.errorClient.Report(errorreporting.Entry{
		Error: err,
	})
	errLogger.Print(err)
}
