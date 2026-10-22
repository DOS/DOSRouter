# CLI process acceptance

`smoke-cli.mjs` runs the real Windows CLI against a synthetic HTTP gateway
bound to `127.0.0.1`. It requires Node.js 26, the Go toolchain, Git and a
clean-source CLI binary with Go VCS metadata. No packages are installed.

Build the binary with `-buildvcs=true -trimpath` and set
`github.com/DOS/DOSRouter/proxy.Version` to the first seven characters of
the source revision. Supply its absolute path through `--binary` and a
disposable evidence directory through `--output`. `--revision` can pin
an earlier source commit; otherwise the harness requires the current HEAD.
The embedded revision must match and `vcs.modified` must be false.

The harness creates three isolated USERPROFILE directories below the ignored
`.smoke-artifacts` directory. It clears API-key/proxy overrides and uses only
synthetic spending state. It never contacts a real provider. It is Windows-only
because USERPROFILE isolation would not protect a Unix user's HOME.

The suite takes just over five minutes because it observes the real catalog
refresh timer. It writes `upstream-smoke.json` and a Vietnamese HTML report.
Without `--serve`, it stops its own children and exits nonzero for a failed
scenario or incomplete shutdown. Browser acceptance remains explicitly pending.

With `--serve`, the process prints localhost fixture, proxy and report URLs and
stays available for independent browser testing. Use the browser's same-origin
fetch for health, models and chat; cross-site navigation must return 403. Only
after those observations, POST the observed proof to the fixture's `/__finish`
endpoint. This records that proof and stops the CLI children. POST `/__stop` to
close the remaining fixture. Ctrl+C also stops owned children. These control
endpoints exist only in this synthetic test fixture, never in the product.

The proof must identify MCP Playwright, the expected health revision, catalog,
same-origin chat model/status, fetch-site and cross-site status. Its chat must
match a `browser-same-origin` request in the independent gateway trace; otherwise
the finish endpoint returns 422 and the report cannot pass browser acceptance.
Health/catalog/browser-guard observations remain external driver attestations,
not facts that the upstream gateway can independently observe.

Reports must distinguish synthetic process acceptance from live provider or
deployment evidence. Review corrections to the test driver separately and keep
the original failed observation alongside any same-binary recheck.
