# Package Manager Infrastructure

Infrastructure used by the automated package publishing pipeline (see
`docs/superpowers/plans/2026-08-05-package-managers.md`).

## Repositories

| Repository | Purpose |
|---|---|
| `accreation/homebrew-tap` | Homebrew tap (formulae live in `Formula/`) |
| `accreation/scoop-bucket` | Scoop bucket (manifests live in `bucket/`) |
| `accreation/aide-repo` | APT/DNF package repository; GitHub Pages site served from `gh-pages` branch (`https://accreation.github.io/aide-repo/`) |

All three are public and pushable by CI via dedicated deploy keys.

## GPG Signing Key

- Key ID: `D1112AAA6FFB3F87`
- Fingerprint: `C1CB40F312E664CCDF985DB9D1112AAA6FFB3F87`
- Algorithm: RSA 4096, no expiry, no passphrase
- UID: `Aide Package Signing <aide@accreation.com>`
- Public key: `https://accreation.github.io/aide-repo/gpg.key` (also on the
  `gh-pages` branch of `accreation/aide-repo` as `gpg.key`)

The private key is stored in the `GPG_PRIVATE_KEY` repository secret of
`accreation/aide` (armored ASCII, `-----BEGIN PGP PRIVATE KEY BLOCK-----`).

## GitHub Secrets (repo: `accreation/aide`)

| Secret | Purpose | Status |
|---|---|---|
| `GPG_PRIVATE_KEY` | Armored GPG private key for APT/DNF signing | Set |
| `GPG_PASSPHRASE` | GPG passphrase (empty — key has none) | Set (empty) |
| `HOMEBREW_TAP_DEPLOY_KEY` | SSH deploy key (write) for `accreation/homebrew-tap` | Set |
| `SCOOP_BUCKET_DEPLOY_KEY` | SSH deploy key (write) for `accreation/scoop-bucket` | Set |
| `AIDE_REPO_DEPLOY_KEY` | SSH deploy key (write) for `accreation/aide-repo` | Set |
| `WINGET_GITHUB_TOKEN` | PAT with winget-pkgs write access | **PLACEHOLDER — replace** |
| `CHOCOLATEY_API_KEY` | Chocolatey Community Repository API key | **PLACEHOLDER — replace** |

`WINGET_GITHUB_TOKEN` and `CHOCOLATEY_API_KEY` are set to
`PLACEHOLDER_REPLACE_ME` and must be replaced with real credentials before the
winget/chocolatey publish steps run.

## Deploy Keys

Each repo has a deploy key titled `CI Deploy Key` with write access. The
matching private keys live only in the secrets above and in the task 1 session
artifacts (see task report).

> Note: `accreation` org previously had `deploy_keys_enabled_for_repositories`
> set to `false`, which blocks deploy key creation. It was enabled (`true`)
> during setup.
