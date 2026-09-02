---
name: authara-release
description: Prepare, execute, verify, or diagnose Authara Core and SDK releases using Release Please and immutable Core OpenAPI provenance. Use for version decisions, release PRs or tags, SDK regeneration, release drift, and browser npm publishing.
---

# Authara release workflow

Use the repository state as the authority. Do not rely on remembered version
numbers or assume that a previously described release is still pending.

## Establish the current state

Before changing or releasing anything:

1. Read `RELEASING.md`.
2. Read `CONTRACT.md` when public API, configuration, cookie, redirect, error,
   or webhook behavior is involved.
3. Inspect the Core working tree, recent commits, latest version tag, open
   Release Please PR, and relevant GitHub Actions runs.
4. Read `.release-please-manifest.json`, `release-please-config.json`,
   `.github/workflows/release-please.yaml`, and `.github/workflows/cd.yaml`.
5. When the sibling SDK repositories are available, inspect their working
   trees, release manifests, `.codegen/manifest.json`, sync workflows, release
   workflows, latest tags, and open automation PRs.

In the standard workspace, the SDK repositories are:

- Go: `../authara-sdk/backend/authara-go`
- Browser: `../authara-sdk/browser/authara-browser`

Preserve unrelated or user-authored changes. If a repository or remote state is
unavailable, report that limitation instead of inventing its state.

## Preserve these invariants

- `contract/openapi.yaml` is the source of truth for generated SDK APIs.
- Implement server behavior and its OpenAPI contract together.
- SDKs are generated from the exact released Core tag and commit, never from a
  moving branch such as `main`.
- Core, Go SDK, and browser SDK versions are independent and need not match.
- Release Please owns release version files, changelogs, tags, and GitHub
  releases. Do not bump or tag them manually during the normal flow.
- Do not edit generated SDK files by hand. Change Core's contract or the SDK
  generator, then regenerate.
- A release preparation request does not authorize merging the Core release PR,
  creating a tag, deploying, or publishing. Perform those actions only when the
  user has requested release execution.
- Merging the Core Release Please PR is the deliberate Core release trigger.
  Once authorized, downstream SDK update and release PRs may auto-merge as
  configured.

## Determine version impact

Classify public compatibility with `CONTRACT.md`, then use Conventional Commits:

- `fix:` produces a patch release.
- `feat:` produces a minor release.
- `type!:` or a `BREAKING CHANGE:` footer marks a breaking release.
- While a package is below `v1.0.0`, its configured
  `bump-minor-pre-major` policy turns breaking changes into a minor bump.

Inspect all unreleased commits since the latest tag. The highest-impact commit
determines the proposed release; do not classify from only the most recent
commit. Additive Core API changes do not force matching SDK version numbers.
They cause each changed SDK to select its own next version from its own release
history.

## Prepare a Core change

1. Update `contract/openapi.yaml` for an API contract change.
2. Implement the corresponding server behavior in the same change set.
3. Update `CONTRACT.md` or migration documentation when compatibility promises
   change.
4. Run `make check-generated` and the tests proportionate to the change. Use
   `make test` for the complete database-backed Core suite.
5. Review the final diff for accidental generated output or manual version
   edits.
6. Commit with the Conventional Commit type matching the compatibility impact.
7. Let Release Please create or update the Core release PR on `main`.

Do not modify either SDK merely to copy the unreleased Core contract. The SDK
sync begins only after the Core release tag exists.

## Execute and verify a Core-driven release

1. Confirm Core CI passed and the Release Please PR proposes the intended
   version and release notes.
2. Treat merging that PR as the production release decision. Merge it only when
   release execution is in scope.
3. Verify Release Please created the expected Core tag and GitHub release at the
   merged commit. Do not create a replacement tag manually.
4. Verify `.github/workflows/cd.yaml` built and pushed the tagged and `latest`
   Core images successfully.
5. Verify the CD workflow dispatched `authara-core-released` to both SDKs with
   the exact Core tag, commit SHA, and release bump.
6. In each SDK, verify the sync workflow checked out that exact tag, confirmed
   its SHA, regenerated the SDK, ran its tests, and recorded the source in
   `.codegen/manifest.json`.
7. If generated output or initial provenance changed, verify the automation PR
   contains only the expected generated files and provenance update and that CI
   passed.
8. Verify each SDK's Release Please workflow selected a version from that SDK's
   own unreleased commits and created its own tag and GitHub release.
9. For the browser SDK, verify `.github/workflows/publish.yaml` tested and built
   the released commit and that the same version was published to npm with
   provenance.

An SDK may legitimately remain at its existing version when regeneration did
not change its generated code. A provenance-only `chore` update does not by
itself require an SDK release. If an SDK manifest still says `unreleased`, do
not bypass its bootstrap auto-merge guard; release Core first and let the exact
tag update the manifest.

## Release handwritten SDK changes

Handwritten SDK code and generator changes use the SDK repository's independent
Release Please flow and do not require a new Core release.

1. Read that SDK's `.codegen/manifest.json` and regenerate against its recorded
   immutable Core commit when generator output is affected.
2. Run its drift check, formatting or linting, tests, and build as applicable.
3. Commit the source or generator change and regenerated output together using
   the correct Conventional Commit type.
4. Let the SDK's Release Please workflow choose and publish its next version.

Never change generated output without also changing the source contract or
generator that reproducibly creates it.

## Recover safely

- If Core deployed but an SDK dispatch failed, rerun the SDK's `Sync released
  Core API` workflow with the already released `core_tag`, its exact 40-character
  `core_sha`, and the original `core_bump`. Do not substitute `main` or a newer
  commit.
- If an automation PR fails drift checks, fix the Core contract or SDK generator
  and regenerate; do not patch generated files to make CI green.
- If auto-merge is unavailable, leave the tested PR for manual merge rather
  than bypassing branch protection.
- Never move, overwrite, or delete a published tag or package version as routine
  recovery. Correct the problem in a subsequent release unless the user
  explicitly authorizes an exceptional rollback procedure.
- After retries, verify the tag, commit, manifest provenance, GitHub release,
  container image, and npm version agree before declaring the release complete.

The shared `AUTHARA_SDK_RELEASE_TOKEN`, SDK auto-merge settings and required CI
checks, and the browser npm Trusted Publisher configuration are operational
prerequisites. Report a missing prerequisite clearly; do not weaken the release
workflow to work around it.
