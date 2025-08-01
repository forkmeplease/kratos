// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package webauthnx

import (
	_ "embed"
	"net/http"

	"github.com/ory/kratos/x"
)

//go:embed js/webauthn.js
var jsOnLoad []byte

const ScriptURL = "/.well-known/ory/webauthn.js"

// swagger:model webAuthnJavaScript
//
//nolint:deadcode,unused
//lint:ignore U1000 Used to generate Swagger and OpenAPI definitions
type webAuthnJavaScript string

// swagger:route GET /.well-known/ory/webauthn.js frontend getWebAuthnJavaScript
//
// # Get WebAuthn JavaScript
//
// This endpoint provides JavaScript which is needed in order to perform WebAuthn login and registration.
//
// If you are building a JavaScript Browser App (e.g. in ReactJS or AngularJS) you will need to load this file:
//
//	```html
//	<script src="https://public-kratos.example.org/.well-known/ory/webauthn.js" type="script" async />
//	```
//
// More information can be found at [Ory Kratos User Login](https://www.ory.sh/docs/kratos/self-service/flows/user-login) and [User Registration Documentation](https://www.ory.sh/docs/kratos/self-service/flows/user-registration).
//
//	Produces:
//	- text/javascript
//
//	Schemes: http, https
//
//	Responses:
//	  200: webAuthnJavaScript
func RegisterWebauthnRoute(r *x.RouterPublic) {
	if !r.HasRoute("GET", ScriptURL) {
		r.GET(ScriptURL, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=UTF-8")
			_, _ = w.Write(jsOnLoad)
		})
	}
}
