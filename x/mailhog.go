// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package x

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"

	"github.com/ory/dockertest/v3"
)

var (
	resourceMux sync.Mutex
	resources   []*dockertest.Resource
)

func CleanUpTestSMTP() {
	resourceMux.Lock()
	defer resourceMux.Unlock()
	for _, resource := range resources {
		resource.Close()
	}
	resources = nil
}

func RunTestSMTP(options ...string) (smtp, api string, err error) {
	if smtp, api := os.Getenv("TEST_MAILHOG_SMTP"), os.Getenv("TEST_MAILHOG_API"); smtp != "" && api != "" {
		return smtp, api, nil
	} else if len(smtp)+len(api) > 0 {
		return "", "", errors.New("environment variables TEST_MAILHOG_SMTP, TEST_MAILHOG_API must both be set!")
	}

	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", "", err
	}
	if err := pool.Client.Ping(); err != nil {
		return "", "", err
	}

	if len(options) == 0 {
		options = []string{
			"-invite-jim",
			"-jim-linkspeed-affect=0.05",
			"-jim-reject-auth=0.05",
			"-jim-reject-recipient=0.05",
			"-jim-reject-sender=0.05",
			"-jim-disconnect=0.05",
			"-jim-linkspeed-min=1250",
			"-jim-linkspeed-max=12500",
		}
	}

	resource, err := pool.
		RunWithOptions(&dockertest.RunOptions{
			Repository: "mailhog/mailhog",
			Tag:        "v1.0.0",
			Cmd:        options,
		})
	if err != nil {
		return "", "", err
	}
	apiPort := resource.GetPort("8025/tcp")
	smtpPort := resource.GetPort("1025/tcp")
	resourceMux.Lock()
	resources = append(resources, resource)
	resourceMux.Unlock()

	smtp = fmt.Sprintf("smtp://test:test@127.0.0.1:%s/?disable_starttls=true", smtpPort)
	api = fmt.Sprintf("http://127.0.0.1:%s", apiPort)
	if err := backoff.Retry(func() error {
		res, err := http.Get(api + "/api/v2/messages")
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			err := errors.Errorf("expected status code 200 but got: %d", res.StatusCode)
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 15)); err != nil {
		return "", "", err
	}

	return smtp, api, nil
}
