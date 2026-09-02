# Authara Core agent guide

## Release and SDK work

For any task involving versions, releases, release pull requests, tags,
publishing, generated SDKs, OpenAPI compatibility, or release automation, read
and follow [the Authara release skill](.agents/skills/authara-release/SKILL.md)
before taking action.

Treat these files as the canonical repository state:

- `CONTRACT.md` defines public compatibility and the required semantic-version
  impact of contract changes.
- `RELEASING.md` documents the release architecture and repository settings.
- `contract/openapi.yaml` is the source of truth for generated SDK APIs.
- Release Please manifests, configuration, and GitHub workflows define the
  current versions and executable release process.

Do not manually create release tags, edit generated SDK output, or bypass the
Core-to-SDK provenance flow. Preparing a release does not by itself authorize
merging a release pull request or publishing artifacts.
