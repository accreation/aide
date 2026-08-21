# Credential-profile verification runbook (macOS / Windows)

Tracks issue [#41](https://github.com/accreation/aide/issues/41) Q3: everything about
credential-profile isolation (`internal/account`) was established on Linux. This is the
procedure to actually run on a macOS or Windows machine to confirm or refute it — nothing
here is a substitute for running it. Update the "macOS/Windows isolation is unverified"
caveat in `README.md`'s Multi-Account Switching section (and this file's Results log)
once you have.

## Prerequisites

- `aide` built for the target OS (`make build`, or a release binary).
- Two real, distinct accounts per provider under test — two Claude subscriptions, two
  GitHub accounts with Copilot access, two opencode-supported provider logins. Reusing
  one account twice cannot detect a shared-store bug (both logins would look identical).
- The provider CLI itself installed (`claude`, `gh` + the Copilot CLI extension,
  `opencode`) and reachable on `PATH`, except where a step says to remove it.

## Generic two-account procedure

Run once per provider, per OS:

```bash
aide account add test-a --provider <claude|codex|copilot|opencode>
aide account add test-b --provider <claude|codex|copilot|opencode>
aide account login test-a   # interactive login as account A
aide account status test-a  # note the identity it reports
aide account login test-b   # interactive login as account B
aide account status test-a  # MUST still report account A — this is the check that matters
aide account status test-b  # MUST report account B
```

The failure mode this is designed to catch is a shared store where logging into B
silently overwrites or invalidates A. `aide account status` alone isn't sufficient
confirmation if the two providers happen to expose the same probe both before and after
— also launch a real project bound to each account (`account: test-a` / `test-b` in
`aide.yaml`) and confirm the provider itself (not just aide's identity probe) is
operating as the right user — e.g. `claude`'s own `/status`, or a Copilot completion that
reveals the org/plan.

Then let both sit idle past whatever the provider's access-token TTL is (an hour is a
reasonable default for OAuth access tokens; check the provider's docs if known) and
repeat both `status` calls — a refresh is a second, separate write to the credential
store and can hit a shared-store bug that a fresh login didn't.

## Provider-specific things to inspect

**`claude`** — credentials should end up keyed by a hash of each profile's absolute
`CLAUDE_CONFIG_DIR`, not shared:
- macOS: `security find-generic-password -s "Claude Code-credentials-<hash>"` (or
  Keychain Access.app) — expect two distinct service names, one per profile, each
  resolvable by `security find-generic-password -w`.
- Windows: expect either `<profile>\claude\.credentials.json` on disk, or two distinct
  Credential Manager entries (`cmdkey /list` or Credential Manager UI) if
  `tengu_windows_credman` is on for the installed version.
- Also confirm `claude`'s background daemon behavior noted in `README.md` — with
  `CLAUDE_CONFIG_DIR` set, does `claude --bg` (or whatever surfaces the daemon in the
  installed version) refuse to start? aide never invokes it, so a refusal here is
  expected and not a bug — just confirm it doesn't also break the plain foreground
  session aide does launch.

**`copilot`** — expected to be the one that fails. Its own credential store is
process-global on macOS/Windows with no directory-scoped override, so `COPILOT_HOME`
likely does not separate two accounts at copilot's own storage layer even though
`GH_CONFIG_DIR` separates `gh`'s:
- Check where copilot's own token lives on each OS (Keychain entry name, or a file
  under the user profile) and whether logging into profile B's `gh` account changes
  what that store contains.
- Remove (or rename) `gh` from `PATH` and re-run the identity check — tier 5 of
  copilot's fallback chain (`gh auth token`) disappears; confirm what aide's
  `copilotIdentity` reports in that state (expected: `LoggedIn: false`, not a crash) and
  what the real `copilot` CLI does when actually launched.

**`opencode`** — confirm the emergent, undocumented behavior actually holds:
- Confirm `<XDG_DATA_HOME>/opencode/auth.json` is really where the installed
  `opencode` version writes credentials (not some other path under `~/.local/share` or
  a platform-specific dir it falls back to when `XDG_DATA_HOME` is respected on
  non-Linux).
- Deliberately point `XDG_DATA_HOME` at a directory the opencode version doesn't
  support (or an old/new opencode release) and confirm the check *fails closed*
  (`aide account status` reports not-logged-in) rather than silently falling back to a
  shared, unisolated location.

## Recording results

Once run, update:
1. The two caveat blockquotes in `README.md` under "Multi-Account Switching" — replace
   "unverified" with the actual verified behavior (or a confirmed limitation) per OS.
2. This file's Results log below.
3. If a provider's isolation genuinely doesn't hold (e.g. copilot's global store), file
   a scoped bug issue rather than just updating the docs — a known-broken isolation
   guarantee needs a decision on remediation (block the platform? warn at login time?),
   which is out of scope for a doc fix.

### Results log

| Provider | OS | Date | Result | Notes |
|----------|-----|------|--------|-------|
| claude   | macOS | — | not yet run | |
| claude   | Windows | — | not yet run | |
| copilot  | macOS | — | not yet run | |
| copilot  | Windows | — | not yet run | |
| opencode | macOS | — | not yet run | |
| opencode | Windows | — | not yet run | |
| opencode | Linux | — | not yet run | unverified per #41, even though isolation was implemented there |
