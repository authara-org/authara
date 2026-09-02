# Releases

Authara Core and its SDKs are versioned independently. The OpenAPI contract in
`contract/openapi.yaml` is the source of truth for generated SDK code.

## Release flow

1. Merge Core changes using Conventional Commit messages.
2. Release Please creates or updates the Core release pull request.
3. Merge the release pull request when the Core release is ready.
4. The resulting Core tag deploys the image and dispatches the immutable tag,
   commit, and release type to the Go and browser SDK repositories.
5. Each SDK regenerates from that exact Core tag, tests the result, and opens an
   update pull request when its generated output changed.
6. SDK update and release pull requests are configured for auto-merge after
   their required checks pass. Each SDK is then tagged with its own version.
7. Browser releases are published to npm from the Release Please workflow.

During the initial rollout, SDK release pull requests can be prepared but are
not auto-merged until `.codegen/manifest.json` records a tagged Core release.
This prevents the pending SDK versions from being published before their Core
source release exists.

Handwritten SDK changes are released independently by the Release Please
workflow in that SDK repository. They do not require a Core release.

## Versioning

- `fix:` creates a patch release.
- `feat:` creates a minor release.
- A commit with `!` or a `BREAKING CHANGE:` footer creates a breaking release.
- While the projects are below `v1.0.0`, breaking changes bump the minor
  version because `bump-minor-pre-major` is enabled.

Core and SDK version numbers do not need to match. SDK generation provenance is
recorded in each SDK repository's `.codegen/manifest.json`.

## Repository settings

The `AUTHARA_SDK_RELEASE_TOKEN` secret must be available to all three
repositories. It needs access to create pull requests, push automation branches,
create releases, and dispatch workflows across the Authara repositories.

Enable pull-request auto-merge in both SDK repositories and make their CI job a
required check. If auto-merge is unavailable, the automation leaves a tested PR
ready for manual merge instead of bypassing branch protection.

The npm Trusted Publisher must authorize
`.github/workflows/publish.yaml` in `authara-org/authara-browser`.

## Generated files

Do not edit generated SDK files directly. Change the Core OpenAPI contract or
the appropriate SDK generator, then regenerate. SDK CI checks the committed
files against the immutable Core commit recorded in the generation manifest.
