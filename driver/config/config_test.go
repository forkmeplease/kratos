// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ory/x/contextx"

	"github.com/ory/x/httpx"
	"github.com/ory/x/randx"

	"github.com/ory/x/snapshotx"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"github.com/ory/kratos/internal/testhelpers"

	"github.com/ory/x/configx"

	"github.com/sirupsen/logrus/hooks/test"

	"github.com/ory/x/logrusx"
	"github.com/ory/x/urlx"

	_ "github.com/ory/jsonschema/v3/fileloader"

	"github.com/ory/kratos/driver/config"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViperProvider(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("suite=loaders", func(t *testing.T) {
		p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.WithConfigFiles("stub/.kratos.yaml"), configx.WithContext(ctx))

		t.Run("group=client config", func(t *testing.T) {
			assert.False(t, p.ClientHTTPNoPrivateIPRanges(ctx), "Should not have private IP ranges disabled per default")
			assert.Equal(t, []string{}, p.ClientHTTPPrivateIPExceptionURLs(ctx), "Should return the correct exceptions")

			p.MustSet(ctx, config.ViperKeyClientHTTPNoPrivateIPRanges, true)
			assert.True(t, p.ClientHTTPNoPrivateIPRanges(ctx), "Should disallow private IP ranges if set")

			p.MustSet(ctx, config.ViperKeyClientHTTPPrivateIPExceptionURLs, []string{"https://foobar.com/baz"})
			assert.Equal(t, []string{"https://foobar.com/baz"}, p.ClientHTTPPrivateIPExceptionURLs(ctx), "Should return the correct exceptions")
		})

		t.Run("group=urls", func(t *testing.T) {
			assert.Equal(t, "http://test.kratos.ory.sh/login", p.SelfServiceFlowLoginUI(ctx).String())
			assert.Equal(t, "http://test.kratos.ory.sh/settings", p.SelfServiceFlowSettingsUI(ctx).String())
			assert.Equal(t, "http://test.kratos.ory.sh/register", p.SelfServiceFlowRegistrationUI(ctx).String())
			assert.Equal(t, "http://test.kratos.ory.sh/error", p.SelfServiceFlowErrorURL(ctx).String())

			assert.Equal(t, "http://admin.kratos.ory.sh", p.SelfAdminURL(ctx).String())
			assert.Equal(t, "http://public.kratos.ory.sh", p.SelfPublicURL(ctx).String())

			var ds []string
			for _, v := range p.SelfServiceBrowserAllowedReturnToDomains(ctx) {
				ds = append(ds, v.String())
			}

			assert.Equal(t, []string{
				"http://return-to-1-test.ory.sh/",
				"http://return-to-2-test.ory.sh/",
				"http://*.wildcards.ory.sh",
				"/return-to-relative-test/",
			}, ds)

			pWithFragments := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.WithValues(map[string]interface{}{
				config.ViperKeySelfServiceLoginUI:        "http://test.kratos.ory.sh/#/login",
				config.ViperKeySelfServiceSettingsURL:    "http://test.kratos.ory.sh/#/settings",
				config.ViperKeySelfServiceRegistrationUI: "http://test.kratos.ory.sh/#/register",
				config.ViperKeySelfServiceErrorUI:        "http://test.kratos.ory.sh/#/error",
			}), configx.SkipValidation())

			assert.Equal(t, "http://test.kratos.ory.sh/#/login", pWithFragments.SelfServiceFlowLoginUI(ctx).String())
			assert.Equal(t, "http://test.kratos.ory.sh/#/settings", pWithFragments.SelfServiceFlowSettingsUI(ctx).String())
			assert.Equal(t, "http://test.kratos.ory.sh/#/register", pWithFragments.SelfServiceFlowRegistrationUI(ctx).String())
			assert.Equal(t, "http://test.kratos.ory.sh/#/error", pWithFragments.SelfServiceFlowErrorURL(ctx).String())

			pWithRelativeFragments := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.WithValues(map[string]interface{}{
				config.ViperKeySelfServiceLoginUI:        "/login",
				config.ViperKeySelfServiceSettingsURL:    "/settings",
				config.ViperKeySelfServiceRegistrationUI: "/register",
				config.ViperKeySelfServiceErrorUI:        "/error",
			}), configx.SkipValidation())

			assert.Equal(t, "/login", pWithRelativeFragments.SelfServiceFlowLoginUI(ctx).String())
			assert.Equal(t, "/settings", pWithRelativeFragments.SelfServiceFlowSettingsUI(ctx).String())
			assert.Equal(t, "/register", pWithRelativeFragments.SelfServiceFlowRegistrationUI(ctx).String())
			assert.Equal(t, "/error", pWithRelativeFragments.SelfServiceFlowErrorURL(ctx).String())

			for _, v := range []string{
				"#/login",
				"test.kratos.ory.sh/login",
			} {
				logger := logrusx.New("", "")
				logger.Logger.ExitFunc = func(code int) { panic("") }
				hook := new(test.Hook)
				logger.Logger.Hooks.Add(hook)

				pWithIncorrectUrls := config.MustNew(t, logger, &contextx.Default{}, configx.WithValues(map[string]interface{}{
					config.ViperKeySelfServiceLoginUI: v,
				}), configx.SkipValidation())

				assert.Panics(t, func() { pWithIncorrectUrls.SelfServiceFlowLoginUI(ctx) })

				assert.Equal(t, logrus.FatalLevel, hook.LastEntry().Level)
				assert.Equal(t, "Configuration value from key selfservice.flows.login.ui_url is not a valid URL: "+v, hook.LastEntry().Message)
				assert.Equal(t, 1, len(hook.Entries))
			}
		})

		t.Run("group=default_return_to", func(t *testing.T) {
			assert.Equal(t, "https://self-service/login/password/return_to", p.SelfServiceFlowLoginReturnTo(ctx, "password").String())
			assert.Equal(t, "https://self-service/login/return_to", p.SelfServiceFlowLoginReturnTo(ctx, "oidc").String())

			assert.Equal(t, "https://self-service/registration/return_to", p.SelfServiceFlowRegistrationReturnTo(ctx, "password").String())
			assert.Equal(t, "https://self-service/registration/oidc/return_to", p.SelfServiceFlowRegistrationReturnTo(ctx, "oidc").String())

			assert.Equal(t, "https://self-service/settings/password/return_to", p.SelfServiceFlowSettingsReturnTo(ctx, "password", p.SelfServiceBrowserDefaultReturnTo(ctx)).String())
			assert.Equal(t, "https://self-service/settings/return_to", p.SelfServiceFlowSettingsReturnTo(ctx, "profile", p.SelfServiceBrowserDefaultReturnTo(ctx)).String())

			assert.Equal(t, "http://test.kratos.ory.sh:4000/", p.SelfServiceFlowLogoutRedirectURL(ctx).String())
			p.MustSet(ctx, config.ViperKeySelfServiceLogoutBrowserDefaultReturnTo, "")
			assert.Equal(t, "http://return-to-3-test.ory.sh/", p.SelfServiceFlowLogoutRedirectURL(ctx).String())
		})

		t.Run("group=identity", func(t *testing.T) {
			c := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.WithConfigFiles("stub/.kratos.mock.identities.yaml"), configx.SkipValidation())

			ds, err := c.DefaultIdentityTraitsSchemaURL(ctx)
			require.NoError(t, err)
			assert.Equal(t, "http://test.kratos.ory.sh/default-identity.schema.json", ds.String())

			ss, err := c.IdentityTraitsSchemas(ctx)
			require.NoError(t, err)
			assert.Equal(t, 2, len(ss))

			assert.Contains(t, ss, config.Schema{
				ID:  "default",
				URL: "http://test.kratos.ory.sh/default-identity.schema.json",
			})
			assert.Contains(t, ss, config.Schema{
				ID:  "other",
				URL: "http://test.kratos.ory.sh/other-identity.schema.json",
			})
		})

		t.Run("group=serve", func(t *testing.T) {
			admin := p.ServeAdmin(ctx)
			assert.Equal(t, "admin.kratos.ory.sh", admin.Host)
			assert.Equal(t, 1234, admin.Port)

			public := p.ServePublic(ctx)
			assert.Equal(t, "public.kratos.ory.sh", public.Host)
			assert.Equal(t, 1235, public.Port)
		})

		t.Run("group=dsn", func(t *testing.T) {
			assert.Equal(t, "sqlite://foo.db?mode=memory&_fk=true", p.DSN(ctx))
		})

		t.Run("group=secrets", func(t *testing.T) {
			assert.Equal(t, [][]byte{
				[]byte("session-key-7f8a9b77-1"),
				[]byte("session-key-7f8a9b77-2"),
			}, p.SecretsSession(ctx))
			var cipherExpected [32]byte
			for k, v := range []byte("secret-thirty-two-character-long") {
				cipherExpected[k] = v
			}
			assert.Equal(t, [][32]byte{
				cipherExpected,
			}, p.SecretsCipher(ctx))
		})

		t.Run("group=methods", func(t *testing.T) {
			for _, tc := range []struct {
				id      string
				config  string
				enabled bool
			}{
				{id: "password", enabled: true, config: `{"haveibeenpwned_host":"api.pwnedpasswords.com","haveibeenpwned_enabled":true,"ignore_network_errors":true,"max_breaches":0,"migrate_hook":{"config":{"emit_analytics_event":true,"method":"POST"},"enabled":false},"min_password_length":8,"identifier_similarity_check_enabled":true}`},
				{id: "oidc", enabled: true, config: `{"providers":[{"client_id":"a","client_secret":"b","id":"github","provider":"github","mapper_url":"http://test.kratos.ory.sh/default-identity.schema.json"}]}`},
				{id: "totp", enabled: true, config: `{"issuer":"issuer.ory.sh"}`},
			} {
				strategy := p.SelfServiceStrategy(ctx, tc.id)
				assert.Equal(t, tc.enabled, strategy.Enabled)
				assert.JSONEq(t, tc.config, string(strategy.Config))
			}
		})

		t.Run("method=registration", func(t *testing.T) {
			assert.Equal(t, true, p.SelfServiceFlowRegistrationEnabled(ctx))
			assert.Equal(t, time.Minute*98, p.SelfServiceFlowRegistrationRequestLifespan(ctx))

			t.Run("hook=before", func(t *testing.T) {
				expHooks := []config.SelfServiceHook{
					{Name: "web_hook", Config: json.RawMessage(`{"headers":{"X-Custom-Header":"test"},"method":"GET","url":"https://test.kratos.ory.sh/before_registration_hook"}`)},
					{Name: "two_step_registration", Config: json.RawMessage(`{}`)},
				}

				hooks := p.SelfServiceFlowRegistrationBeforeHooks(ctx)

				assert.Equal(t, expHooks, hooks)
				// assert.EqualValues(t, "redirect", hook.Name)
				// assert.JSONEq(t, `{"allow_user_defined_redirect":false,"default_redirect_url":"http://test.kratos.ory.sh:4000/"}`, string(hook.Config))
			})

			for _, tc := range []struct {
				strategy string
				hooks    []config.SelfServiceHook
			}{
				{
					strategy: "password",
					hooks: []config.SelfServiceHook{
						{Name: "session", Config: json.RawMessage(`{}`)},
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_registration_password_hook"}`)},
						// {Name: "verify", Config: json.RawMessage(`{}`)},
						// {Name: "redirect", Config: json.RawMessage(`{"allow_user_defined_redirect":false,"default_redirect_url":"http://test.kratos.ory.sh:4000/"}`)},
					},
				},
				{
					strategy: "oidc",
					hooks: []config.SelfServiceHook{
						// {Name: "verify", Config: json.RawMessage(`{}`)},
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"GET","url":"https://test.kratos.ory.sh/after_registration_oidc_hook"}`)},
						{Name: "session", Config: json.RawMessage(`{}`)},
						// {Name: "redirect", Config: json.RawMessage(`{"allow_user_defined_redirect":false,"default_redirect_url":"http://test.kratos.ory.sh:4000/"}`)},
					},
				},
				{
					strategy: config.HookGlobal,
					hooks: []config.SelfServiceHook{
						{Name: "web_hook", Config: json.RawMessage(`{"auth":{"config":{"in":"header","name":"My-Key","value":"My-Key-Value"},"type":"api_key"},"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_registration_global_hook"}`)},
					},
				},
			} {
				t.Run("hook=after/strategy="+tc.strategy, func(t *testing.T) {
					hooks := p.SelfServiceFlowRegistrationAfterHooks(ctx, tc.strategy)
					assert.Equal(t, tc.hooks, hooks)
				})
			}
		})

		t.Run("method=totp", func(t *testing.T) {
			assert.Equal(t, "issuer.ory.sh", p.TOTPIssuer(ctx))
		})

		t.Run("method=login", func(t *testing.T) {
			assert.Equal(t, time.Minute*99, p.SelfServiceFlowLoginRequestLifespan(ctx))

			t.Run("hook=before", func(t *testing.T) {
				expHooks := []config.SelfServiceHook{
					{Name: "web_hook", Config: json.RawMessage(`{"headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/before_login_hook"}`)},
				}

				hooks := p.SelfServiceFlowLoginBeforeHooks(ctx)

				require.Len(t, hooks, 1)
				assert.Equal(t, expHooks, hooks)
				// assert.EqualValues(t, "redirect", hook.Name)
				// assert.JSONEq(t, `{"allow_user_defined_redirect":false,"default_redirect_url":"http://test.kratos.ory.sh:4000/"}`, string(hook.Config))
			})

			for _, tc := range []struct {
				strategy string
				hooks    []config.SelfServiceHook
			}{
				{
					strategy: "password",
					hooks: []config.SelfServiceHook{
						{Name: "revoke_active_sessions", Config: json.RawMessage(`{}`)},
						{Name: "require_verified_address", Config: json.RawMessage(`{}`)},
						{Name: "web_hook", Config: json.RawMessage(`{"auth":{"config":{"password":"super-secret","user":"test-user"},"type":"basic_auth"},"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_login_password_hook"}`)},
					},
				},
				{
					strategy: "oidc",
					hooks: []config.SelfServiceHook{
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"GET","url":"https://test.kratos.ory.sh/after_login_oidc_hook"}`)},
						{Name: "revoke_active_sessions", Config: json.RawMessage(`{}`)},
					},
				},
				{
					strategy: config.HookGlobal,
					hooks: []config.SelfServiceHook{
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_login_global_hook"}`)},
					},
				},
			} {
				t.Run("hook=after/strategy="+tc.strategy, func(t *testing.T) {
					hooks := p.SelfServiceFlowLoginAfterHooks(ctx, tc.strategy)
					assert.Equal(t, tc.hooks, hooks)
				})
			}
		})

		t.Run("method=settings", func(t *testing.T) {
			assert.Equal(t, time.Minute*99, p.SelfServiceFlowSettingsFlowLifespan(ctx))
			assert.Equal(t, time.Minute*5, p.SelfServiceFlowSettingsPrivilegedSessionMaxAge(ctx))

			for _, tc := range []struct {
				strategy string
				hooks    []config.SelfServiceHook
			}{
				{
					strategy: "password",
					hooks: []config.SelfServiceHook{
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_settings_password_hook"}`)},
					},
				},
				{
					strategy: "profile",
					hooks: []config.SelfServiceHook{
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_settings_profile_hook"}`)},
					},
				},
				{
					strategy: config.HookGlobal,
					hooks: []config.SelfServiceHook{
						{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"POST","url":"https://test.kratos.ory.sh/after_settings_global_hook"}`)},
					},
				},
			} {
				t.Run("hook=after/strategy="+tc.strategy, func(t *testing.T) {
					hooks := p.SelfServiceFlowSettingsAfterHooks(ctx, tc.strategy)
					assert.Equal(t, tc.hooks, hooks)
				})
			}
		})

		t.Run("method=recovery", func(t *testing.T) {
			assert.Equal(t, true, p.SelfServiceFlowRecoveryEnabled(ctx))
			assert.Equal(t, time.Minute*98, p.SelfServiceFlowRecoveryRequestLifespan(ctx))
			assert.Equal(t, "http://test.kratos.ory.sh/recovery", p.SelfServiceFlowRecoveryUI(ctx).String())

			hooks := p.SelfServiceFlowRecoveryAfterHooks(ctx, config.HookGlobal)
			assert.Equal(t, []config.SelfServiceHook{{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"GET","url":"https://test.kratos.ory.sh/after_recovery_hook"}`)}}, hooks)
		})

		t.Run("method=verification", func(t *testing.T) {
			assert.Equal(t, time.Minute*97, p.SelfServiceFlowVerificationRequestLifespan(ctx))
			assert.Equal(t, "http://test.kratos.ory.sh/verification", p.SelfServiceFlowVerificationUI(ctx).String())

			hooks := p.SelfServiceFlowVerificationAfterHooks(ctx, config.HookGlobal)
			assert.Equal(t, []config.SelfServiceHook{{Name: "web_hook", Config: json.RawMessage(`{"body":"/path/to/template.jsonnet","headers":{"X-Custom-Header":"test"},"method":"GET","url":"https://test.kratos.ory.sh/after_verification_hook"}`)}}, hooks)
		})

		t.Run("group=hashers", func(t *testing.T) {
			c := p.HasherArgon2(ctx)
			assert.Equal(t, &config.Argon2{
				Memory: 1048576, Iterations: 2, Parallelism: 4,
				SaltLength: 16, KeyLength: 32, DedicatedMemory: config.Argon2DefaultDedicatedMemory, ExpectedDeviation: config.Argon2DefaultDeviation, ExpectedDuration: config.Argon2DefaultDuration,
			}, c)
		})

		t.Run("group=set_provider_by_json", func(t *testing.T) {
			providerConfigJSON := `{"providers": [{"id":"github-test","provider":"github","client_id":"set_json_test","client_secret":"secret","mapper_url":"http://mapper-url","scope":["user:email"]}]}`
			strategyConfigJSON := fmt.Sprintf(`{"enabled":true, "config": %s}`, providerConfigJSON)

			p.MustSet(ctx, config.ViperKeySelfServiceStrategyConfig+".oidc", strategyConfigJSON)
			strategy := p.SelfServiceStrategy(ctx, "oidc")
			assert.JSONEq(t, providerConfigJSON, string(strategy.Config))
		})
	})
}

func TestBcrypt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())

	require.NoError(t, p.Set(ctx, config.ViperKeyHasherBcryptCost, 4))
	require.NoError(t, p.Set(ctx, "dev", false))
	assert.EqualValues(t, uint32(12), p.HasherBcrypt(ctx).Cost)

	require.NoError(t, p.Set(ctx, "dev", true))
	assert.EqualValues(t, uint32(4), p.HasherBcrypt(ctx).Cost)
}

func TestProviderBaseURLs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	machineHostname, err := os.Hostname()
	if err != nil {
		machineHostname = "127.0.0.1"
	}

	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())
	assert.Equal(t, "https://"+machineHostname+":4433/", p.SelfPublicURL(ctx).String())
	assert.Equal(t, "https://"+machineHostname+":4434/", p.SelfAdminURL(ctx).String())

	p = config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation(), configx.WithValues(map[string]interface{}{
		"serve.public.port": 4444,
		"serve.admin.port":  4445,
	}))
	assert.Equal(t, "https://"+machineHostname+":4444/", p.SelfPublicURL(ctx).String())
	assert.Equal(t, "https://"+machineHostname+":4445/", p.SelfAdminURL(ctx).String())

	p = config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation(), configx.WithValues(map[string]interface{}{
		"serve.public.host": "public.ory.sh",
		"serve.admin.host":  "admin.ory.sh",
		"serve.public.port": 4444,
		"serve.admin.port":  4445,
	}))
	assert.Equal(t, "https://public.ory.sh:4444/", p.SelfPublicURL(ctx).String())
	assert.Equal(t, "https://admin.ory.sh:4445/", p.SelfAdminURL(ctx).String())

	// Set to dev mode
	p = config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation(), configx.WithValues(map[string]interface{}{
		"serve.public.host": "public.ory.sh",
		"serve.admin.host":  "admin.ory.sh",
		"serve.public.port": 4444,
		"serve.admin.port":  4445,
		"dev":               true,
	}))
	assert.Equal(t, "http://public.ory.sh:4444/", p.SelfPublicURL(ctx).String())
	assert.Equal(t, "http://admin.ory.sh:4445/", p.SelfAdminURL(ctx).String())
}

func TestProviderSelfServiceLinkMethodBaseURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	machineHostname, err := os.Hostname()
	if err != nil {
		machineHostname = "127.0.0.1"
	}

	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())
	assert.Equal(t, "https://"+machineHostname+":4433/", p.SelfServiceLinkMethodBaseURL(ctx).String())

	p.MustSet(ctx, config.ViperKeyLinkBaseURL, "https://example.org/bar")
	assert.Equal(t, "https://example.org/bar", p.SelfServiceLinkMethodBaseURL(ctx).String())
}

func TestDefaultWebhookHeaderAllowlist(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())
	snapshotx.SnapshotT(t, p.WebhookHeaderAllowlist(ctx))
}

func TestViperProvider_Secrets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())

	def := p.SecretsDefault(ctx)
	assert.NotEmpty(t, def)
	assert.Equal(t, def, p.SecretsSession(ctx))
	assert.Equal(t, def, p.SecretsDefault(ctx))
	assert.Empty(t, p.SecretsCipher(ctx))
	err := p.Set(ctx, config.ViperKeySecretsCipher, []string{"short-secret-key"})
	require.NoError(t, err)
	assert.Equal(t, [][32]byte{}, p.SecretsCipher(ctx))
}

func TestViperProvider_Defaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrusx.New("", "")

	for k, tc := range []struct {
		init   func() *config.Config
		expect func(t *testing.T, p *config.Config)
	}{
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())
			},
		},
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.WithConfigFiles("stub/.defaults.yml"), configx.SkipValidation())
			},
		},
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.WithConfigFiles("stub/.defaults-password.yml"), configx.SkipValidation())
			},
		},
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.WithConfigFiles("../../test/e2e/profiles/recovery/.kratos.yml"), configx.SkipValidation())
			},
			expect: func(t *testing.T, p *config.Config) {
				assert.True(t, p.SelfServiceFlowRecoveryEnabled(ctx))
				assert.False(t, p.SelfServiceFlowVerificationEnabled(ctx))
				assert.True(t, p.SelfServiceFlowRegistrationEnabled(ctx))
				assert.True(t, p.SelfServiceStrategy(ctx, "password").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "profile").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "link").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "code").Enabled)
				assert.False(t, p.SelfServiceCodeStrategy(ctx).PasswordlessEnabled)
				assert.False(t, p.SelfServiceStrategy(ctx, "oidc").Enabled)
			},
		},
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.WithConfigFiles("../../test/e2e/profiles/verification/.kratos.yml"), configx.SkipValidation())
			},
			expect: func(t *testing.T, p *config.Config) {
				assert.False(t, p.SelfServiceFlowRecoveryEnabled(ctx))
				assert.True(t, p.SelfServiceFlowVerificationEnabled(ctx))
				assert.True(t, p.SelfServiceFlowRegistrationEnabled(ctx))
				assert.True(t, p.SelfServiceStrategy(ctx, "password").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "profile").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "link").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "code").Enabled)
				assert.False(t, p.SelfServiceCodeStrategy(ctx).PasswordlessEnabled)
				assert.False(t, p.SelfServiceStrategy(ctx, "oidc").Enabled)
			},
		},
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.WithConfigFiles("../../test/e2e/profiles/oidc/.kratos.yml"), configx.SkipValidation())
			},
			expect: func(t *testing.T, p *config.Config) {
				assert.False(t, p.SelfServiceFlowRecoveryEnabled(ctx))
				assert.False(t, p.SelfServiceFlowVerificationEnabled(ctx))
				assert.True(t, p.SelfServiceStrategy(ctx, "password").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "profile").Enabled)
				assert.False(t, p.SelfServiceStrategy(ctx, "link").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "code").Enabled)
				assert.True(t, p.SelfServiceStrategy(ctx, "oidc").Enabled)
				assert.False(t, p.SelfServiceCodeStrategy(ctx).PasswordlessEnabled)
			},
		},
		{
			init: func() *config.Config {
				return config.MustNew(t, l, &contextx.Default{}, configx.WithConfigFiles("stub/.kratos.notify-unknown-recipients.yml"), configx.SkipValidation())
			},
			expect: func(t *testing.T, p *config.Config) {
				assert.True(t, p.SelfServiceFlowRecoveryNotifyUnknownRecipients(ctx))
				assert.True(t, p.SelfServiceFlowVerificationNotifyUnknownRecipients(ctx))
			},
		},
	} {
		t.Run(fmt.Sprintf("case=%d", k), func(t *testing.T) {
			p := tc.init()

			if tc.expect != nil {
				tc.expect(t, p)
				return
			}
			assert.False(t, p.SelfServiceFlowRecoveryEnabled(ctx))
			assert.False(t, p.SelfServiceFlowVerificationEnabled(ctx))
			assert.True(t, p.SelfServiceStrategy(ctx, "password").Enabled)
			assert.True(t, p.SelfServiceStrategy(ctx, "profile").Enabled)
			assert.False(t, p.SelfServiceStrategy(ctx, "link").Enabled)
			assert.True(t, p.SelfServiceStrategy(ctx, "code").Enabled)
			assert.False(t, p.SelfServiceStrategy(ctx, "oidc").Enabled)
			assert.False(t, p.SelfServiceCodeStrategy(ctx).PasswordlessEnabled)
			assert.False(t, p.SelfServiceFlowRecoveryNotifyUnknownRecipients(ctx))
			assert.False(t, p.SelfServiceFlowVerificationNotifyUnknownRecipients(ctx))
		})
	}

	t.Run("suite=ui_url", func(t *testing.T) {
		p := config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())
		assert.Equal(t, "https://www.ory.sh/kratos/docs/fallback/login", p.SelfServiceFlowLoginUI(ctx).String())
		assert.Equal(t, "https://www.ory.sh/kratos/docs/fallback/settings", p.SelfServiceFlowSettingsUI(ctx).String())
		assert.Equal(t, "https://www.ory.sh/kratos/docs/fallback/registration", p.SelfServiceFlowRegistrationUI(ctx).String())
		assert.Equal(t, "https://www.ory.sh/kratos/docs/fallback/recovery", p.SelfServiceFlowRecoveryUI(ctx).String())
		assert.Equal(t, "https://www.ory.sh/kratos/docs/fallback/verification", p.SelfServiceFlowVerificationUI(ctx).String())
	})
}

func TestViperProvider_ReturnTo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrusx.New("", "")
	p := config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())

	p.MustSet(ctx, config.ViperKeySelfServiceBrowserDefaultReturnTo, "https://www.ory.sh/")
	assert.Equal(t, "https://www.ory.sh/", p.SelfServiceFlowVerificationReturnTo(ctx, urlx.ParseOrPanic("https://www.ory.sh/")).String())
	assert.Equal(t, "https://www.ory.sh/", p.SelfServiceFlowRecoveryReturnTo(ctx, urlx.ParseOrPanic("https://www.ory.sh/")).String())

	p.MustSet(ctx, config.ViperKeySelfServiceRecoveryBrowserDefaultReturnTo, "https://www.ory.sh/recovery")
	assert.Equal(t, "https://www.ory.sh/recovery", p.SelfServiceFlowRecoveryReturnTo(ctx, urlx.ParseOrPanic("https://www.ory.sh/")).String())

	p.MustSet(ctx, config.ViperKeySelfServiceVerificationBrowserDefaultReturnTo, "https://www.ory.sh/verification")
	assert.Equal(t, "https://www.ory.sh/verification", p.SelfServiceFlowVerificationReturnTo(ctx, urlx.ParseOrPanic("https://www.ory.sh/")).String())
}

func TestSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrusx.New("", "")
	p := config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())

	assert.Equal(t, "ory_kratos_session", p.SessionName(ctx))
	p.MustSet(ctx, config.ViperKeySessionName, "ory_session")
	assert.Equal(t, "ory_session", p.SessionName(ctx))

	assert.Equal(t, time.Hour*24, p.SessionRefreshMinTimeLeft(ctx))
	p.MustSet(ctx, config.ViperKeySessionRefreshMinTimeLeft, "1m")
	assert.Equal(t, time.Minute, p.SessionRefreshMinTimeLeft(ctx))

	assert.Equal(t, time.Hour*24, p.SessionLifespan(ctx))
	p.MustSet(ctx, config.ViperKeySessionLifespan, "1m")
	assert.Equal(t, time.Minute, p.SessionLifespan(ctx))

	assert.Equal(t, true, p.SessionPersistentCookie(ctx))
	p.MustSet(ctx, config.ViperKeySessionPersistentCookie, false)
	assert.Equal(t, false, p.SessionPersistentCookie(ctx))

	assert.Equal(t, false, p.SessionWhoAmICaching(ctx))
	p.MustSet(ctx, config.ViperKeySessionWhoAmICaching, true)
	assert.Equal(t, true, p.SessionWhoAmICaching(ctx))
}

func TestCookies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrusx.New("", "")
	p := config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())

	t.Run("path", func(t *testing.T) {
		assert.Equal(t, "/", p.CookiePath(ctx))
		assert.Equal(t, "/", p.SessionPath(ctx))

		p.MustSet(ctx, config.ViperKeyCookiePath, "/cookie")
		assert.Equal(t, "/cookie", p.CookiePath(ctx))
		assert.Equal(t, "/cookie", p.SessionPath(ctx))

		p.MustSet(ctx, config.ViperKeySessionPath, "/session")
		assert.Equal(t, "/cookie", p.CookiePath(ctx))
		assert.Equal(t, "/session", p.SessionPath(ctx))
	})

	t.Run("SameSite", func(t *testing.T) {
		assert.Equal(t, http.SameSiteLaxMode, p.CookieSameSiteMode(ctx))
		assert.Equal(t, http.SameSiteLaxMode, p.SessionSameSiteMode(ctx))

		p.MustSet(ctx, config.ViperKeyCookieSameSite, "Strict")
		assert.Equal(t, http.SameSiteStrictMode, p.CookieSameSiteMode(ctx))
		assert.Equal(t, http.SameSiteStrictMode, p.SessionSameSiteMode(ctx))

		p.MustSet(ctx, config.ViperKeySessionSameSite, "None")
		assert.Equal(t, http.SameSiteStrictMode, p.CookieSameSiteMode(ctx))
		assert.Equal(t, http.SameSiteNoneMode, p.SessionSameSiteMode(ctx))
	})

	t.Run("domain", func(t *testing.T) {
		assert.Equal(t, "", p.CookieDomain(ctx))
		assert.Equal(t, "", p.SessionDomain(ctx))

		p.MustSet(ctx, config.ViperKeyCookieDomain, "www.cookie.com")
		assert.Equal(t, "www.cookie.com", p.CookieDomain(ctx))
		assert.Equal(t, "www.cookie.com", p.SessionDomain(ctx))

		p.MustSet(ctx, config.ViperKeySessionDomain, "www.session.com")
		assert.Equal(t, "www.cookie.com", p.CookieDomain(ctx))
		assert.Equal(t, "www.session.com", p.SessionDomain(ctx))
	})
}

func TestViperProvider_DSN(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=dsn: memory", func(t *testing.T) {
		p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())
		p.MustSet(ctx, config.ViperKeyDSN, "memory")

		assert.Equal(t, config.DefaultSQLiteMemoryDSN, p.DSN(ctx))
	})

	t.Run("case=dsn: not memory", func(t *testing.T) {
		p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())

		dsn := "sqlite://foo.db?_fk=true"
		p.MustSet(ctx, config.ViperKeyDSN, dsn)

		assert.Equal(t, dsn, p.DSN(ctx))
	})

	t.Run("case=dsn: not set", func(t *testing.T) {
		dsn := ""

		var exitCode int
		l := logrusx.New("", "", logrusx.WithExitFunc(func(i int) {
			exitCode = i
		}))
		p := config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())

		assert.Equal(t, dsn, p.DSN(ctx))
		assert.NotEqual(t, 0, exitCode)
	})
}

func TestViperProvider_ParseURIOrFail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var exitCode int

	l := logrusx.New("", "", logrusx.WithExitFunc(func(i int) {
		exitCode = i
	}))
	p := config.MustNew(t, l, &contextx.Default{}, configx.SkipValidation())
	require.Zero(t, exitCode)

	const testKey = "testKeyNotUsedInTheRealSchema"

	for _, tc := range []struct {
		u        string
		expected url.URL
	}{
		{
			u: "file:///etc/config/kratos/identity.schema.json",
			expected: url.URL{
				Scheme: "file",
				Path:   "/etc/config/kratos/identity.schema.json",
			},
		},
		{
			u: "file://./identity.schema.json",
			expected: url.URL{
				Scheme: "file",
				Host:   ".",
				Path:   "/identity.schema.json",
			},
		},
		{
			u: "base64://bG9jYWwgc3ViamVjdCA9I",
			expected: url.URL{
				Scheme: "base64",
				Host:   "bG9jYWwgc3ViamVjdCA9I",
			},
		},
		{
			u: "https://foo.bar/schema.json",
			expected: url.URL{
				Scheme: "https",
				Host:   "foo.bar",
				Path:   "/schema.json",
			},
		},
	} {
		t.Run("case=parse "+tc.u, func(t *testing.T) {
			require.NoError(t, p.Set(ctx, testKey, tc.u))

			u := p.ParseURIOrFail(ctx, testKey)
			require.Zero(t, exitCode)
			assert.Equal(t, tc.expected, *u)
		})
	}
}

func TestViperProvider_HaveIBeenPwned(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.SkipValidation())
	t.Run("case=hipb: host", func(t *testing.T) {
		p.MustSet(ctx, config.ViperKeyPasswordHaveIBeenPwnedHost, "foo.bar")
		assert.Equal(t, "foo.bar", p.PasswordPolicyConfig(ctx).HaveIBeenPwnedHost)
	})

	t.Run("case=hibp: enabled", func(t *testing.T) {
		p.MustSet(ctx, config.ViperKeyPasswordHaveIBeenPwnedEnabled, true)
		assert.Equal(t, true, p.PasswordPolicyConfig(ctx).HaveIBeenPwnedEnabled)
	})

	t.Run("case=hibp: enabled", func(t *testing.T) {
		p.MustSet(ctx, config.ViperKeyPasswordHaveIBeenPwnedEnabled, false)
		assert.Equal(t, false, p.PasswordPolicyConfig(ctx).HaveIBeenPwnedEnabled)
	})

	t.Run("case=hibp: max_breaches", func(t *testing.T) {
		p.MustSet(ctx, config.ViperKeyPasswordMaxBreaches, 10)
		assert.Equal(t, uint(10), p.PasswordPolicyConfig(ctx).MaxBreaches)
	})

	t.Run("case=hibp: ignore_network_errors", func(t *testing.T) {
		p.MustSet(ctx, config.ViperKeyIgnoreNetworkErrors, true)
		assert.Equal(t, true, p.PasswordPolicyConfig(ctx).IgnoreNetworkErrors)
	})

	t.Run("case=hibp: ignore_network_errors", func(t *testing.T) {
		p.MustSet(ctx, config.ViperKeyIgnoreNetworkErrors, false)
		assert.Equal(t, false, p.PasswordPolicyConfig(ctx).IgnoreNetworkErrors)
	})
}

func newTestConfig(t *testing.T, opts ...configx.OptionModifier) (c *config.Config, l *logrusx.Logger, h *test.Hook, exited *bool) {
	l = logrusx.New("", "")
	h = new(test.Hook)
	exited = new(bool)
	l.Logger.Hooks.Add(h)
	l.Logger.ExitFunc = func(code int) { *exited = true }
	c = config.MustNew(t, l, &contextx.Default{}, append([]configx.OptionModifier{configx.SkipValidation()}, opts...)...)
	return
}

func TestLoadingTLSConfig(t *testing.T) {
	t.Parallel()

	certPath, keyPath, certBase64, keyBase64 := testhelpers.GenerateTLSCertificateFilesForTests(t)

	t.Run("case=public: no TLS config", func(t *testing.T) {
		p, l, hook, exited := newTestConfig(t)
		certFunc, err := p.ServePublic(t.Context()).TLS.GetCertFunc(t.Context(), l, "public")
		require.NoError(t, err)
		assert.Nil(t, certFunc)
		le := hook.LastEntry()
		require.NotNil(t, le)
		assert.Equal(t, "TLS has not been configured for public, skipping", le.Message)
		assert.False(t, *exited)
	})

	t.Run("case=admin: no TLS config", func(t *testing.T) {
		p, l, hook, exited := newTestConfig(t)
		certFunc, err := p.ServeAdmin(t.Context()).TLS.GetCertFunc(t.Context(), l, "admin")
		require.NoError(t, err)
		assert.Nil(t, certFunc)
		le := hook.LastEntry()
		require.NotNil(t, le)
		assert.Equal(t, "TLS has not been configured for admin, skipping", le.Message)
		assert.False(t, *exited)
	})

	t.Run("case=public: loading inline base64 certificate", func(t *testing.T) {
		p, l, hook, exited := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyPublicTLSKeyBase64:  keyBase64,
			keyPublicTLSCertBase64: certBase64,
		}))
		certFunc, err := p.ServePublic(t.Context()).TLS.GetCertFunc(t.Context(), l, "public")
		require.NoError(t, err)
		assert.NotNil(t, certFunc)
		le := hook.LastEntry()
		require.NotNil(t, le)
		assert.Equal(t, "Setting up HTTPS for public", le.Message)
		assert.False(t, *exited)
	})

	t.Run("case=public: loading certificate from a file", func(t *testing.T) {
		p, l, hook, exited := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyPublicTLSKeyPath:  keyPath,
			keyPublicTLSCertPath: certPath,
		}))
		certFunc, err := p.ServePublic(t.Context()).TLS.GetCertFunc(t.Context(), l, "public")
		require.NoError(t, err)
		assert.NotNil(t, certFunc)
		le := hook.LastEntry()
		require.NotNil(t, le)
		assert.Equal(t, "Setting up HTTPS for public (automatic certificate reloading active)", le.Message)
		assert.False(t, *exited)
	})

	t.Run("case=public: failing to load inline base64 certificate", func(t *testing.T) {
		p, l, _, _ := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyPublicTLSKeyBase64:  "invalid",
			keyPublicTLSCertBase64: certBase64,
		}))
		certFunc, err := p.ServePublic(t.Context()).TLS.GetCertFunc(t.Context(), l, "public")
		require.ErrorContains(t, err, "unable to load TLS certificate for interface public")
		assert.Nil(t, certFunc)
	})

	t.Run("case=public: failing to load certificate from a file", func(t *testing.T) {
		p, l, _, _ := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyPublicTLSKeyPath:  "/dev/null",
			keyPublicTLSCertPath: "/dev/null",
		}))
		certFunc, err := p.ServePublic(t.Context()).TLS.GetCertFunc(t.Context(), l, "public")
		require.ErrorContains(t, err, "unable to load TLS certificate for interface public")
		assert.Nil(t, certFunc)
	})

	t.Run("case=admin: loading inline base64 certificate", func(t *testing.T) {
		p, l, hook, exited := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyAdminTLSKeyBase64:  keyBase64,
			keyAdminTLSCertBase64: certBase64,
		}))
		certFunc, err := p.ServeAdmin(t.Context()).TLS.GetCertFunc(t.Context(), l, "admin")
		require.NoError(t, err)
		assert.NotNil(t, certFunc)
		le := hook.LastEntry()
		require.NotNil(t, le)
		assert.Equal(t, "Setting up HTTPS for admin", le.Message)
		assert.False(t, *exited)
	})

	t.Run("case=admin: loading certificate from a file", func(t *testing.T) {
		p, l, hook, exited := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyAdminTLSKeyPath:  keyPath,
			keyAdminTLSCertPath: certPath,
		}))
		certFunc, err := p.ServeAdmin(t.Context()).TLS.GetCertFunc(t.Context(), l, "admin")
		require.NoError(t, err)
		assert.NotNil(t, certFunc)
		le := hook.LastEntry()
		require.NotNil(t, le)
		assert.Equal(t, "Setting up HTTPS for admin (automatic certificate reloading active)", le.Message)
		assert.False(t, *exited)
	})

	t.Run("case=admin: failing to load inline base64 certificate", func(t *testing.T) {
		p, l, _, _ := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyAdminTLSKeyBase64:  "invalid",
			keyAdminTLSCertBase64: certBase64,
		}))
		certFunc, err := p.ServeAdmin(t.Context()).TLS.GetCertFunc(t.Context(), l, "admin")
		assert.Nil(t, certFunc)
		require.ErrorContains(t, err, "unable to load TLS certificate for interface admin")
	})

	t.Run("case=admin: failing to load certificate from a file", func(t *testing.T) {
		p, l, _, _ := newTestConfig(t, configx.WithValues(map[string]interface{}{
			keyAdminTLSKeyPath:  "/dev/null",
			keyAdminTLSCertPath: certPath,
		}))
		certFunc, err := p.ServeAdmin(t.Context()).TLS.GetCertFunc(t.Context(), l, "admin")
		require.ErrorContains(t, err, "unable to load TLS certificate for interface admin")
		assert.Nil(t, certFunc)
	})
}

func TestIdentitySchemaValidation(t *testing.T) {
	t.Parallel()
	files := []string{"stub/.identity.test.json", "stub/.identity.other.json"}

	ctx := context.Background()
	ctx = config.SetValidateIdentitySchemaResilientClientOptions(ctx, []httpx.ResilientOptions{
		httpx.ResilientClientWithMaxRetry(0),
		httpx.ResilientClientWithConnectionTimeout(time.Millisecond * 100),
	})

	type identity struct {
		Schemas []map[string]string `json:"schemas"`
	}

	type configFile struct {
		identityFileName string
		SelfService      map[string]string            `json:"selfservice"`
		Courier          map[string]map[string]string `json:"courier"`
		DSN              string                       `json:"dsn"`
		Identity         *identity                    `json:"identity"`
	}

	setup := func(t *testing.T, file string) *configFile {
		identityTest, err := os.ReadFile(file)
		assert.NoError(t, err)
		return &configFile{
			identityFileName: file,
			SelfService: map[string]string{
				"default_browser_return_url": "https://some-return-url",
			},
			Courier: map[string]map[string]string{
				"smtp": {
					"connection_uri": "smtp://foo@bar",
				},
			},
			DSN: "memory",
			Identity: &identity{
				Schemas: []map[string]string{{"id": "default", "url": "base64://" + base64.StdEncoding.EncodeToString(identityTest)}},
			},
		}
	}

	marshalAndWrite := func(t *testing.T, ctx context.Context, tmpFile *os.File, identity *configFile) {
		j, err := yaml.Marshal(identity)
		assert.NoError(t, err)

		_, err = tmpFile.Seek(0, 0)
		require.NoError(t, err)
		require.NoError(t, tmpFile.Truncate(0))
		_, err = io.WriteString(tmpFile, string(j))
		assert.NoError(t, err)
		assert.NoError(t, tmpFile.Sync())
	}

	testWatch := func(t *testing.T, ctx context.Context, cmd *cobra.Command, identity *configFile) (*config.Config, *test.Hook, func([]map[string]string)) {
		tdir := t.TempDir()
		assert.NoError(t,
			os.MkdirAll(tdir,
				os.ModePerm))
		configFileName := randx.MustString(8, randx.Alpha)
		tmpConfig, err := os.Create(filepath.Join(tdir, configFileName+".config.yaml"))
		assert.NoError(t, err)
		t.Cleanup(func() { tmpConfig.Close() })

		marshalAndWrite(t, ctx, tmpConfig, identity)

		l := logrusx.New("kratos-"+tmpConfig.Name(), "test")
		hook := test.NewLocal(l.Logger)

		conf, err := config.New(ctx, l, os.Stderr, &contextx.Default{}, configx.WithConfigFiles(tmpConfig.Name()))
		assert.NoError(t, err)

		// clean the hooks since it will throw an event on first boot
		hook.Reset()

		return conf, hook, func(schemas []map[string]string) {
			identity.Identity.Schemas = schemas
			marshalAndWrite(t, ctx, tmpConfig, identity)
		}
	}

	t.Run("case=skip invalid schema validation", func(t *testing.T) {
		_, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.invalid.identities.yaml"),
			configx.SkipValidation())
		assert.NoError(t, err)
	})

	t.Run("case=invalid schema should throw error", func(t *testing.T) {
		var stdErr bytes.Buffer
		_, err := config.New(ctx, logrusx.New("", ""), &stdErr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.invalid.identities.yaml"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "minimum 1 properties allowed, but found 0")
		assert.Contains(t, stdErr.String(), "minimum 1 properties allowed, but found 0")
	})

	t.Run("case=must fail on loading unreachable schemas", func(t *testing.T) {
		// we make sure that the test runs into DNS issues instead of the context being canceled
		ctx := config.SetValidateIdentitySchemaResilientClientOptions(ctx, []httpx.ResilientOptions{
			httpx.ResilientClientWithMaxRetry(0),
			httpx.ResilientClientWithConnectionTimeout(5 * time.Second),
		})

		err := make(chan error)
		go func(err chan error) {
			_, e := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
				configx.WithConfigFiles("stub/.kratos.mock.identities.yaml"))
			err <- e
		}(err)

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("the test could not complete as the context timed out before the identity schema loader timed out")
		case e := <-err:
			assert.ErrorContains(t, e, "no such host")
		}
	})

	t.Run("case=validate schema is validated on file change", func(t *testing.T) {
		var identities []*configFile

		for _, f := range files {
			identities = append(identities, setup(t, f))
		}

		invalidIdentity := setup(t, "stub/.identity.invalid.json")

		for _, identity := range identities {
			t.Run("test=identity file "+identity.identityFileName, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, time.Second*30)
				t.Cleanup(cancel)

				_, hook, writeSchema := testWatch(t, ctx, &cobra.Command{}, identity)
				writeSchema(invalidIdentity.Identity.Schemas)

				// There are a bunch of log messages beeing logged. We are looking for a specific one.
				for {
					for _, v := range hook.AllEntries() {
						s, err := v.String()
						require.NoError(t, err)
						if strings.Contains(s, "The changed identity schema configuration is invalid and could not be loaded.") {
							return
						}
					}
					select {
					case <-ctx.Done():
						t.Fatal("the test could not complete as the context timed out before the file watcher updated")
					default: // nothing
					}
				}
			})
		}
	})
}

func TestPasswordless(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conf, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
		configx.SkipValidation(),
		configx.WithValue(config.ViperKeyWebAuthnPasswordless, true))
	require.NoError(t, err)

	assert.True(t, conf.WebAuthnForPasswordless(ctx))
	conf.MustSet(ctx, config.ViperKeyWebAuthnPasswordless, false)
	assert.False(t, conf.WebAuthnForPasswordless(ctx))
}

func TestPasswordlessCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	conf, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
		configx.SkipValidation(),
		configx.WithValue(config.ViperKeySelfServiceStrategyConfig+".code", map[string]interface{}{
			"passwordless_enabled":                true,
			"passwordless_login_fallback_enabled": true,
			"config":                              map[string]interface{}{},
		}))
	require.NoError(t, err)

	assert.True(t, conf.SelfServiceCodeStrategy(ctx).PasswordlessEnabled)
}

func TestChangeMinPasswordLength(t *testing.T) {
	t.Parallel()
	t.Run("case=must fail on minimum password length below enforced minimum", func(t *testing.T) {
		ctx := context.Background()

		_, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.yaml"),
			configx.WithValue(config.ViperKeyPasswordMinLength, 5))

		assert.Error(t, err)
	})

	t.Run("case=must not fail on minimum password length above enforced minimum", func(t *testing.T) {
		ctx := context.Background()

		_, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.yaml"),
			configx.WithValue(config.ViperKeyPasswordMinLength, 9))

		assert.NoError(t, err)
	})
}

func TestCourierEmailHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=configs set", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.courier.email.http.yaml"), configx.SkipValidation())
		assert.Equal(t, "http", conf.CourierEmailStrategy(ctx))
		snapshotx.SnapshotT(t, conf.CourierEmailRequestConfig(ctx))
	})

	t.Run("case=defaults", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.SkipValidation())

		assert.Equal(t, "smtp", conf.CourierEmailStrategy(ctx))
	})
}

func TestCourierChannels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("case=configs set", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.WithConfigFiles("stub/.kratos.courier.channels.yaml"), configx.SkipValidation())

		channelConfig, err := conf.CourierChannels(ctx)
		require.NoError(t, err)
		require.Len(t, channelConfig, 2)
		assert.Equal(t, channelConfig[0].ID, "phone")
		assert.NotEmpty(t, channelConfig[0].RequestConfig)
		assert.Equal(t, channelConfig[1].ID, "email")
		assert.NotEmpty(t, channelConfig[1].SMTPConfig)
	})

	t.Run("case=defaults", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.SkipValidation())

		channelConfig, err := conf.CourierChannels(ctx)
		require.NoError(t, err)
		assert.Len(t, channelConfig, 1)
		assert.Equal(t, channelConfig[0].ID, "email")
		assert.Equal(t, channelConfig[0].Type, "smtp")
	})

	t.Run("smtp urls", func(t *testing.T) {
		for _, tc := range []string{
			"smtp://a:basdasdasda%2Fc@email-smtp.eu-west-3.amazonaws.com:587/",
			"smtp://a:b$c@email-smtp.eu-west-3.amazonaws.com:587/",
			"smtp://a/a:bc@email-smtp.eu-west-3.amazonaws.com:587",
			"smtp://aa:b+c@email-smtp.eu-west-3.amazonaws.com:587/",
			"smtp://user?name:password@email-smtp.eu-west-3.amazonaws.com:587/",
			"smtp://username:pass%2Fword@email-smtp.eu-west-3.amazonaws.com:587/",
		} {
			t.Run("case="+tc, func(t *testing.T) {
				conf, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.WithValue(config.ViperKeyCourierSMTPURL, tc), configx.SkipValidation())
				require.NoError(t, err)
				cs, err := conf.CourierChannels(ctx)
				require.NoError(t, err)
				require.Len(t, cs, 1)
				assert.Equal(t, tc, cs[0].SMTPConfig.ConnectionURI)
			})
		}
	})
}

func TestCourierMessageTTL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=configs set", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.courier.message_retries.yaml"), configx.SkipValidation())
		assert.Equal(t, conf.CourierMessageRetries(ctx), 10)
	})

	t.Run("case=defaults", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.SkipValidation())
		assert.Equal(t, conf.CourierMessageRetries(ctx), 5)
	})
}

func TestTwoStep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=nothing is set", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.SkipValidation())

		assert.True(t, conf.SelfServiceFlowRegistrationTwoSteps(ctx))
	})

	t.Run("case=legacy config explicit off", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithValue(config.ViperKeySelfServiceRegistrationEnableLegacyOneStep, false),
			configx.SkipValidation(),
		)

		assert.True(t, conf.SelfServiceFlowRegistrationTwoSteps(ctx))
	})

	t.Run("case=legacy config explicit on", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithValue(config.ViperKeySelfServiceRegistrationEnableLegacyOneStep, true),
			configx.SkipValidation(),
		)

		assert.False(t, conf.SelfServiceFlowRegistrationTwoSteps(ctx))
	})

	t.Run("case=new config explicit on", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithValue(config.ViperKeySelfServiceRegistrationFlowStyle, "profile_first"),
			configx.SkipValidation(),
		)

		assert.True(t, conf.SelfServiceFlowRegistrationTwoSteps(ctx))
	})

	t.Run("case=new config explicit off", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithValue(config.ViperKeySelfServiceRegistrationFlowStyle, "unified"),
			configx.SkipValidation(),
		)

		assert.False(t, conf.SelfServiceFlowRegistrationTwoSteps(ctx))
	})

	t.Run("case=new config explicit on but legacy off", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithValue(config.ViperKeySelfServiceRegistrationFlowStyle, "profile_first"),
			configx.WithValue(config.ViperKeySelfServiceRegistrationEnableLegacyOneStep, true),
			configx.SkipValidation(),
		)

		assert.False(t, conf.SelfServiceFlowRegistrationTwoSteps(ctx))
	})
}

func TestOAuth2Provider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=configs set", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.oauth2_provider.yaml"), configx.SkipValidation())
		assert.Equal(t, "https://oauth2_provider/", conf.OAuth2ProviderURL(ctx).String())
		assert.Equal(t, http.Header{"Authorization": {"Basic"}}, conf.OAuth2ProviderHeader(ctx))
		assert.True(t, conf.OAuth2ProviderOverrideReturnTo(ctx))
	})

	t.Run("case=defaults", func(t *testing.T) {
		conf, _ := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{}, configx.SkipValidation())
		assert.Empty(t, conf.OAuth2ProviderURL(ctx))
		assert.Empty(t, conf.OAuth2ProviderHeader(ctx))
		assert.False(t, conf.OAuth2ProviderOverrideReturnTo(ctx))
	})
}

func TestWebauthn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=multiple origins", func(t *testing.T) {
		conf, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.webauthn.origins.yaml"))
		require.NoError(t, err)
		webAuthnConfig := conf.WebAuthnConfig(ctx)
		assert.Equal(t, "https://example.com/webauthn", webAuthnConfig.RPID)
		assert.EqualValues(t, []string{
			"https://origin-a.example.com",
			"https://origin-b.example.com",
			"https://origin-c.example.com",
		}, webAuthnConfig.RPOrigins)
	})

	t.Run("case=one origin", func(t *testing.T) {
		conf, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.webauthn.origin.yaml"))
		require.NoError(t, err)
		webAuthnConfig := conf.WebAuthnConfig(ctx)
		assert.Equal(t, "https://example.com/webauthn", webAuthnConfig.RPID)
		assert.EqualValues(t, []string{
			"https://origin-a.example.com",
		}, webAuthnConfig.RPOrigins)
	})

	t.Run("case=id as origin", func(t *testing.T) {
		conf, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.yaml"))
		require.NoError(t, err)
		webAuthnConfig := conf.WebAuthnConfig(ctx)
		assert.Equal(t, "example.com", webAuthnConfig.RPID)
		assert.EqualValues(t, []string{
			"http://example.com",
		}, webAuthnConfig.RPOrigins)
	})

	t.Run("case=invalid", func(t *testing.T) {
		_, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.webauthn.invalid.yaml"))
		assert.Error(t, err)
	})
}

func TestCourierTemplatesConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("case=partial template update allowed", func(t *testing.T) {
		_, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.courier.remote.partial.templates.yaml"))
		assert.NoError(t, err)
	})

	t.Run("case=load remote template with fallback template overrides path", func(t *testing.T) {
		_, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.courier.remote.templates.yaml"))
		assert.NoError(t, err)
	})

	t.Run("case=courier template helper", func(t *testing.T) {
		c, err := config.New(ctx, logrusx.New("", ""), os.Stderr, &contextx.Default{},
			configx.WithConfigFiles("stub/.kratos.courier.remote.templates.yaml"))

		assert.NoError(t, err)

		courierTemplateConfig := &config.CourierEmailTemplate{
			Body: &config.CourierEmailBodyTemplate{
				PlainText: "",
				HTML:      "",
			},
			Subject: "",
		}

		assert.Equal(t, courierTemplateConfig, c.CourierEmailTemplatesHelper(ctx, config.ViperKeyCourierTemplatesVerificationInvalidEmail))
		assert.Equal(t, courierTemplateConfig, c.CourierEmailTemplatesHelper(ctx, config.ViperKeyCourierTemplatesVerificationValidEmail))
		// this should return an empty courierEmailTemplate as the key does not exist
		assert.Equal(t, courierTemplateConfig, c.CourierEmailTemplatesHelper(ctx, "a_random_key"))

		courierTemplateConfig = &config.CourierEmailTemplate{
			Body: &config.CourierEmailBodyTemplate{
				PlainText: "base64://SGksCgp5b3UgKG9yIHNvbWVvbmUgZWxzZSkgZW50ZXJlZCB0aGlzIGVtYWlsIGFkZHJlc3Mgd2hlbiB0cnlpbmcgdG8gcmVjb3ZlciBhY2Nlc3MgdG8gYW4gYWNjb3VudC4KCkhvd2V2ZXIsIHRoaXMgZW1haWwgYWRkcmVzcyBpcyBub3Qgb24gb3VyIGRhdGFiYXNlIG9mIHJlZ2lzdGVyZWQgdXNlcnMgYW5kIHRoZXJlZm9yZSB0aGUgYXR0ZW1wdCBoYXMgZmFpbGVkLgoKSWYgdGhpcyB3YXMgeW91LCBjaGVjayBpZiB5b3Ugc2lnbmVkIHVwIHVzaW5nIGEgZGlmZmVyZW50IGFkZHJlc3MuCgpJZiB0aGlzIHdhcyBub3QgeW91LCBwbGVhc2UgaWdub3JlIHRoaXMgZW1haWwu",
				HTML:      "base64://SGksCgp5b3UgKG9yIHNvbWVvbmUgZWxzZSkgZW50ZXJlZCB0aGlzIGVtYWlsIGFkZHJlc3Mgd2hlbiB0cnlpbmcgdG8gcmVjb3ZlciBhY2Nlc3MgdG8gYW4gYWNjb3VudC4KCkhvd2V2ZXIsIHRoaXMgZW1haWwgYWRkcmVzcyBpcyBub3Qgb24gb3VyIGRhdGFiYXNlIG9mIHJlZ2lzdGVyZWQgdXNlcnMgYW5kIHRoZXJlZm9yZSB0aGUgYXR0ZW1wdCBoYXMgZmFpbGVkLgoKSWYgdGhpcyB3YXMgeW91LCBjaGVjayBpZiB5b3Ugc2lnbmVkIHVwIHVzaW5nIGEgZGlmZmVyZW50IGFkZHJlc3MuCgpJZiB0aGlzIHdhcyBub3QgeW91LCBwbGVhc2UgaWdub3JlIHRoaXMgZW1haWwu",
			},
			Subject: "base64://QWNjb3VudCBBY2Nlc3MgQXR0ZW1wdGVk",
		}
		assert.Equal(t, courierTemplateConfig, c.CourierEmailTemplatesHelper(ctx, config.ViperKeyCourierTemplatesRecoveryInvalidEmail))

		courierTemplateConfig = &config.CourierEmailTemplate{
			Body: &config.CourierEmailBodyTemplate{
				PlainText: "base64://e3sgZGVmaW5lIGFmLVpBIH19CkhhbGxvLAoKSGVyc3RlbCBqb3UgcmVrZW5pbmcgZGV1ciBoaWVyZGllIHNrYWtlbCB0ZSB2b2xnOgp7ey0gZW5kIC19fQoKe3sgZGVmaW5lIGVuLVVTIH19CkhpLAoKcGxlYXNlIHJlY292ZXIgYWNjZXNzIHRvIHlvdXIgYWNjb3VudCBieSBjbGlja2luZyB0aGUgZm9sbG93aW5nIGxpbms6Cnt7LSBlbmQgLX19Cgp7ey0gaWYgZXEgLmxhbmcgImFmLVpBIiAtfX0KCnt7IHRlbXBsYXRlICJhZi1aQSIgLiB9fQoKe3stIGVsc2UgLX19Cgp7eyB0ZW1wbGF0ZSAiZW4tVVMiIH19Cgp7ey0gZW5kIC19fQp7eyAuUmVjb3ZlcnlVUkwgfX0K",
				HTML:      "base64://e3sgZGVmaW5lIGFmLVpBIH19CkhhbGxvLAoKSGVyc3RlbCBqb3UgcmVrZW5pbmcgZGV1ciBoaWVyZGllIHNrYWtlbCB0ZSB2b2xnOgp7ey0gZW5kIC19fQoKe3sgZGVmaW5lIGVuLVVTIH19CkhpLAoKcGxlYXNlIHJlY292ZXIgYWNjZXNzIHRvIHlvdXIgYWNjb3VudCBieSBjbGlja2luZyB0aGUgZm9sbG93aW5nIGxpbms6Cnt7LSBlbmQgLX19Cgp7ey0gaWYgZXEgLmxhbmcgImFmLVpBIiAtfX0KCnt7IHRlbXBsYXRlICJhZi1aQSIgLiB9fQoKe3stIGVsc2UgLX19Cgp7eyB0ZW1wbGF0ZSAiZW4tVVMiIH19Cgp7ey0gZW5kIC19fQo8YSBocmVmPSJ7eyAuUmVjb3ZlcnlVUkwgfX0iPnt7IC5SZWNvdmVyeVVSTCB9fTwvYT4=",
			},
			Subject: "base64://UmVjb3ZlciBhY2Nlc3MgdG8geW91ciBhY2NvdW50",
		}
		assert.Equal(t, courierTemplateConfig, c.CourierEmailTemplatesHelper(ctx, config.ViperKeyCourierTemplatesRecoveryValidEmail))
	})
}

func TestCleanup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := config.MustNew(t, logrusx.New("", ""), &contextx.Default{}, configx.WithConfigFiles("stub/.kratos.yaml"))

	t.Run("group=cleanup config", func(t *testing.T) {
		assert.Equal(t, p.DatabaseCleanupSleepTables(ctx), 1*time.Minute)
		p.MustSet(ctx, config.ViperKeyDatabaseCleanupSleepTables, "1s")
		assert.Equal(t, p.DatabaseCleanupSleepTables(ctx), time.Second)
		assert.Equal(t, p.DatabaseCleanupBatchSize(ctx), 100)
		p.MustSet(ctx, config.ViperKeyDatabaseCleanupBatchSize, "1")
		assert.Equal(t, p.DatabaseCleanupBatchSize(ctx), 1)
	})
}

const (
	keyPublicTLSCertBase64 = "serve.public.tls.cert.base64"
	keyPublicTLSKeyBase64  = "serve.public.tls.key.base64"
	keyPublicTLSCertPath   = "serve.public.tls.cert.path"
	keyPublicTLSKeyPath    = "serve.public.tls.key.path"
	keyAdminTLSCertBase64  = "serve.admin.tls.cert.base64"
	keyAdminTLSKeyBase64   = "serve.admin.tls.key.base64"
	keyAdminTLSCertPath    = "serve.admin.tls.cert.path"
	keyAdminTLSKeyPath     = "serve.admin.tls.key.path"
)
