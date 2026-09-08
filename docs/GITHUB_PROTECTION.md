# GitHub Repository Protection Baseline

This document records the repository-owner settings required before `main` is treated as protected production history.

The connected GitHub integration used during development can read branch protection but cannot write repository administration settings. Therefore these controls must be applied in GitHub repository settings by an owner.

## Required `main` protection

Configure a branch protection rule or ruleset for `main` with these minimum controls:

- require a pull request before merge;
- require all review conversations to be resolved;
- require the successful checks `Quality, unit, race and config`, `Stress and leak gate`, `Impairment and mini-soak gate`, `Security, reproducibility and release`, and `CodeQL Go`;
- block force pushes;
- block branch deletion;
- prevent direct pushes that bypass the pull-request flow;
- require branches to be up to date before merge when practical;
- do not permit routine bypass of the rule for normal development;
- require a review when a second qualified reviewer exists.

Do not enable a mandatory CODEOWNER approval while the repository has only one effective owner/reviewer if doing so would make the repository impossible to merge safely. Revisit this when a second maintainer exists.

## Security settings

Enable private vulnerability reporting / repository security advisories when available. Keep GitHub Actions workflow permissions at least privilege; workflows in this repository declare explicit minimal permissions and pin third-party actions to immutable commit SHAs.

## Proprietary source visibility

The repository is licensed as proprietary / All Rights Reserved. A public repository can still be read and technically cloned by third parties even though the license does not authorize reuse.

If the operational requirement is not merely “no permission to use” but instead “no public access to the source,” change the repository visibility to **private** before production distribution. A license controls permission; repository privacy controls access.

## Verification record

Before v1.0.0, record in `PROJECT_STATE.md` that these settings were verified against the actual `main` branch. Do not infer protection from the existence of this document or from `CODEOWNERS` alone.
