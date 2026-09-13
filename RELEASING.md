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
4. Release Please creates the version commit, changelog, release tag, and draft
   GitHub release. Do not create or commit any of these manually.
5. A Core release tag checks for numbered SQL migration changes since the
   latest migrations release. If any exist, CD publishes the next `0.X.0`
   migrations image and release; otherwise it keeps the current migrations
   version.
6. CD deploys Core, attaches `authara-images.env`, adds the compatible image
   tags and digests to the release notes, and publishes the completed release.
7. Core CD dispatches the immutable tag, commit, and release type to the Go and
   browser SDK repositories.
8. Each SDK regenerates from that exact Core tag, tests the result, and opens an
   update pull request when its generated output changed.
9. SDK generated-update and release pull requests automatically merge after
   their required checks pass. Each changed SDK is tagged with its own version.
10. Browser releases are published to npm from the Release Please workflow.

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

Migrations are versioned independently. Each automatic migrations release bumps
the minor version and resets the patch version (`0.1.20` → `0.2.0`). A Core
release without numbered SQL migration changes reuses the previous migrations
release.

Core releases remain drafts until CD has built both compatible image references,
attached their machine-readable metadata, and added the same pairing to the
release notes. If CD fails, fix and rerun it; do not publish the incomplete draft
manually.

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
