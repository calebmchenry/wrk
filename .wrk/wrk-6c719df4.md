---
id: wrk-6c719df4
title: Rewrite README for prospective and first-time users
status: done
priority: normal
labels:
  - docs
---
## Outcome

Make the README explain what wrk is, who it helps, why to use it, and how to get from installation to a useful first workflow. Research established README guidance and verify examples against the current CLI.

## Acceptance criteria

- [x] Lead with purpose, audience, benefits, and a short path to first use.
- [x] Explain installation, everyday CLI/browser use, and optional agent/script workflows with accurate examples.
- [x] Link detailed contracts, support, and contributor instructions without duplicating the reference manual.
- [x] Preserve essential install/development instructions and repair any affected documentation links.
- [x] Verify README commands in a disposable project, local links/anchors, and repository validation.

## Research and decisions

- GitHub's README guidance recommends explaining what a project does, why it is useful, how to get started, where to find help, and how to contribute; keep longer reference material elsewhere: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-readmes
- Write the Docs recommends writing for a clear audience, stating the problem solved, providing a small usage example, and making installation/support/contribution paths visible: https://www.writethedocs.org/guide/writing/beginners-guide-to-docs/
- Current README mixes onboarding with detailed implementation/edge-case contracts. Existing docs already cover most of those contracts.
- Initial release check on 2026-10-10 found v0.1.0 without `serve` or `run`. The user subsequently requested writing user documentation for the upcoming release as if already published, with publication handled separately by the user. README and installation guide now follow that direction.
- Preserve the pre-existing unrelated edit to wrk-ef04f589.

## Changes

- Rewrote README around purpose, audience, benefits, source/release installation, a small first workflow, Git storage, organizing work, browser use, agents/scripts, and support.
- Moved detailed release/checksum installation instructions to `docs/install.md` and contributor checks to `docs/development.md`; linked both from README and the documentation index.
- Kept the existing `install-a-release`, `build-and-install`, and `developing-wrk-with-wrk` anchors used by other docs. Detailed CLI/storage/browser/burn contracts remain in their existing reference pages.
- Verified the GitHub repository has Issues enabled before linking the support path. Research references remain above rather than adding README-writing advice to the product README.

## Verification (2026-10-10)

- Installed the current checkout with `go install ./cmd/wrk` into a temporary GOBIN. Ran 38 CLI invocations in a disposable project covering the quick start, body preservation, active/ready/all lists, explicit and nested project selection, parents, dependencies, labels, blocking/unblocking, related links, JSON, verbose runner output, and expected-status/one-ticket execution. All passed; no actual agent was invoked.
- Started `wrk serve --port 0`, fetched its bundled HTML over the printed local URL, and verified Ctrl-C shutdown. Browser auto-launch and visual UI behavior were not exercised; no UI code changed.
- The first ad hoc server check waited on an already-buffered stdout line and timed out. Replaced that harness read with bounded log-file polling; the server check then passed without a product change.
- Checked 95 relative links and anchors across README, the documentation index, and the new install/development guides; all pass. Confirmed the three existing inbound README anchors remain valid.
- `git diff --check` passes. Repository validation reports 41 valid tickets. No production code changed, so no full code test-suite rerun was needed.
- Temporary installation, projects, and server were cleaned up. Preserved the unrelated pre-existing edit to `wrk-ef04f589`.

## Release-ready wording follow-up (2026-10-10)

- At the user's request, removed the old-release caveats and made standalone release installation the primary path, with building from source optional.
- Both user guides link to the latest release. The installation example uses a version placeholder supplied from the release page instead of pinning v0.1.0 or inventing the next release number.
- Verified 95 local links/anchors, retained inbound README anchors, installation shell syntax with `sh -n`, and removal of old-release caveats and version pinning. `git diff --check` passes.
- No release was created or published; the user will handle that separately.

## Remaining work

None for the documentation task.
