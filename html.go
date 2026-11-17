// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

const SwaggerUIHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="description" content="SwaggerUI" />
    <title>Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.35.0/swagger-ui.css" integrity="sha256-XtCHV+U9o+Am1svsxh/l1DWFyVYLMC1TStVmklGaJz0=" crossorigin="anonymous" />
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.35.0/swagger-ui-bundle.js" integrity="sha256-5qiYaS8tdo6AfDukYmBopposXdlg3Wx28+TTo94YCvI=" crossorigin="anonymous"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.35.0/swagger-ui-standalone-preset.js" integrity="sha256-UkPUkuFFBeDKuHrIsBldDmFZQ+ZRdDsraYRQpG60cL4=" crossorigin="anonymous"></script>
    <script>
        window.onload = () => {
            window.ui = SwaggerUIBundle({
                url: 'openapi.yaml',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>
`

const PasskeysPageHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
        body {
            color-scheme: light dark;
            font-family: system-ui, sans-serif;
            max-width: 22rem;
            margin: 4rem auto;
            padding: 0 1rem;
        }

        h1 {
            font-size: 1.4rem;
            text-align: center;
            margin-bottom: 2rem;
        }

        h2 {
            font-size: 1rem;
            margin-top: 0;
        }

        label {
            display: block;
            font-size: .85rem;
            margin: .75rem 0 .25rem;
        }

        input {
            display: block;
            box-sizing: border-box;
            font: inherit;
            width: 100%;
            padding: .4rem .45rem;
            margin-bottom: .75rem;
        }

        button {
            display: block;
            font: inherit;
            width: 80%;
            margin: 1rem auto 0;
            padding: .5rem;
            text-align: center;
            cursor: pointer;
        }

        .auth-section {
            border: 1px solid #888;
            border-radius: 10px;
            padding: 1.25rem;
            margin: 1.5rem 0;
        }

        .auth-section p {
            font-size: .8rem;
            opacity: .7;
            margin: .5rem 0 1rem;
        }

        pre {
            font-size: .7rem;
            white-space: pre-wrap;
            word-break: break-all;
        }
    </style>
</head>

<body>

    <!-- Registration -->
    <section class="auth-section">
        <label>Email
            <input
                id="email"
                type="email"
                autocomplete="email"
            />
        </label>

        <label>Full name
            <input
                id="fullname"
                autocomplete="name"
            />
        </label>

        <button onclick="register()">
						Register
        </button>
    </section>

    <!-- Login -->
    <section class="auth-section">
        <h2>Already registered?</h2>

        <button onclick="login()">
            Sign in
        </button>
    </section>

    <pre id="out"></pre>

    <script>
        const $ = id => document.getElementById(id);

        const out = text =>
            $('out').textContent = text;

        const b64d = s =>
            Uint8Array.from(
                atob(s.replace(/-/g, '+').replace(/_/g, '/')),
                c => c.charCodeAt(0)
            );

        const b64e = b =>
            btoa(
                [...new Uint8Array(b)]
                    .map(c => String.fromCharCode(c))
                    .join('')
            )
            .replace(/\+/g, '-')
            .replace(/\//g, '_')
            .replace(/=+$/, '');

        const post = async (url, body) => {
            const r = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(body)
            });

            const j = await r.json();

            if (!r.ok) throw j;

            return j;
        };

        async function register() {
            try {
                const s = await post(
                    '/webauthn/register/start',
                    {
                        email: $('email').value,
                        full_name: $('fullname').value
                    }
                );

                const pk = s.options.publicKey;

                const c = await navigator.credentials.create({
                    publicKey: {
                        ...pk,
                        challenge: b64d(pk.challenge),
                        user: {
                            ...pk.user,
                            id: b64d(pk.user.id)
                        },
                    },
                });

                const p = await post(
                    '/webauthn/register/finish',
                    {
                        session_id: s.session_id,
                        credential: {
                            id: c.id,
                            rawId: b64e(c.rawId),
                            type: c.type,
                            response: {
                                clientDataJSON:
                                    b64e(c.response.clientDataJSON),
                                attestationObject:
                                    b64e(c.response.attestationObject),
                            },
                        },
                    }
                );

                done(p);

            } catch (e) {
							out(
								e instanceof Error
									? e.message
									: JSON.stringify(e, null, 2)
							);
            }
        }

        async function login() {
            try {
                const s = await post(
                    '/webauthn/login/start',
                    {}
                );

                const pk = s.options.publicKey;

                const c = await navigator.credentials.get({
                    publicKey: {
                        challenge: b64d(pk.challenge),
                        userVerification: pk.userVerification
                    },
                });

                const p = await post(
                    '/webauthn/login/finish',
                    {
                        session_id: s.session_id,
                        credential: {
                            id: c.id,
                            rawId: b64e(c.rawId),
                            type: c.type,
                            response: {
                                clientDataJSON:
                                    b64e(c.response.clientDataJSON),
                                authenticatorData:
                                    b64e(c.response.authenticatorData),
                                signature:
                                    b64e(c.response.signature),
                                userHandle:
                                    b64e(c.response.userHandle),
                            },
                        },
                    }
                );

                done(p);

            } catch (e) {
							out(
								e instanceof Error
									? e.message
									: JSON.stringify(e, null, 2)
							);
            }
        }

        async function done(pair) {
            let who = '';

            try {
                const u = await fetch(
                    '/users/' + pair.user_id,
                    {
                        headers: {
                            Authorization:
                                'Bearer ' + pair.access_token
                        }
                    }
                ).then(r => r.json());

                who =
                    'Signed in as ' +
                    u.full_name +
                    ' (' + u.email + ')\n\n';

            } catch {
							out(
								e instanceof Error
									? e.message
									: JSON.stringify(e, null, 2)
							);
						}

            out(who + JSON.stringify(pair, null, 2));
        }
    </script>

</body>
</html>
`
