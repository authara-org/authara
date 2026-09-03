# Releases

Authara Core and its SDKs are versioned independently. The OpenAPI contract in
`contract/openapi.yaml` is the source of truth for generated SDK code.

## Release flow

1. Give each change pull request a Conventional Commit title such as `fix:`,
   `feat:`, or `feat!:`. CI rejects titles that do not follow this format.
2. After the required checks pass, squash-merge the change pull request. The
   squash commit title is always the pull request title.
3. Release Please calculates the next version and creates or updates its release
   pull request. The release pull request automatically merges after its own
   required checks pass.
4. Release Please creates the version commit, tag, changelog, and GitHub release.
   Do not create or commit any of these manually.
5. A Core release tag deploys the image and dispatches the immutable tag,
   commit, and release type to the Go and browser SDK repositories.
6. Each SDK regenerates from that exact Core tag, tests the result, and opens an
   update pull request when its generated output changed.
7. SDK generated-update and release pull requests automatically merge after
   their required checks pass. Each changed SDK is tagged with its own version.
8. Browser releases are published to npm from the Release Please workflow.

Merging a releasable change pull request is therefore the release decision. No
second manual merge, version edit, or tag command is required. A `docs:`,
`chore:`, `test:`, `build:`, or `ci:` pull request does not create a release by
itself.

During the initial rollout, SDK release pull requests can be prepared but are
not auto-merged until `.codegen/manifest.json` records a tagged Core release.
This prevents the pending SDK versions from being published before their Core
source release exists.

Handwritten SDK changes are released independently by the Release Please
workflow in that SDK repository. They do not require a Core release.

## Versioning

- `fix:` in the pull request title creates a patch release.
- `feat:` in the pull request title creates a minor release.
- A title with `!` or a `BREAKING CHANGE:` footer in the pull request body
  creates a breaking release.
- While the projects are below `v1.0.0`, breaking changes bump the minor
  version because `bump-minor-pre-major` is enabled.

Core and SDK version numbers do not need to match. SDK generation provenance is
recorded in each SDK repository's `.codegen/manifest.json`.

## Repository settings

The `AUTHARA_SDK_RELEASE_TOKEN` secret must be available to all three
repositories. It needs access to create pull requests, push automation branches,
enable auto-merge, create releases, and dispatch workflows across the Authara
repositories.

Enable pull-request auto-merge in Core and both SDK repositories. Require the
`pr-title` check plus each repository's normal CI checks. Release automation
must fail visibly if it cannot enable auto-merge; it must never bypass branch
protection.

The npm Trusted Publisher must authorize
`.github/workflows/publish.yaml` in `authara-org/authara-browser`.

## Generated files

Do not edit generated SDK files directly. Change the Core OpenAPI contract or
the appropriate SDK generator, then regenerate. SDK CI checks the committed
files against the immutable Core commit recorded in the generation manifest.
