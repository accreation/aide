package account

import "strings"

// scrubEnv returns env with every variable named in keys omitted. Omitting
// (rather than setting to "") matters: some tools treat an empty override as
// a distinct, isolation-defeating value (e.g. Claude Code's
// CLAUDE_SECURESTORAGE_CONFIG_DIR="" forces the shared default credential
// store instead of falling through to CLAUDE_CONFIG_DIR).
func scrubEnv(env []string, keys []string) []string {
	if len(keys) == 0 {
		return env
	}
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		key := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			key = e[:i]
		}
		if drop[key] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// replaceEnvVar builds a new env slice, replacing key's value if it already
// exists in env, or appending it otherwise.
func replaceEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, len(env))
	copy(out, env)
	for i, e := range out {
		if strings.HasPrefix(e, prefix) {
			out[i] = prefix + value
			return out
		}
	}
	return append(out, prefix+value)
}

// mergeEnvKV applies each "KEY=VALUE" pair in kvs onto env via
// replaceEnvVar, in order.
func mergeEnvKV(env []string, kvs []string) []string {
	for _, kv := range kvs {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		env = replaceEnvVar(env, kv[:i], kv[i+1:])
	}
	return env
}

// BuildEnv returns the environment for launching acc's provider on top of
// base: base with adapter.Scrub omitted, then adapter.Env(root, acc)
// applied. It is pure — no exec calls, no global mutation — so both the
// launcher and the pre-launch identity check can share it and it can be
// table-tested without a real provider CLI.
func BuildEnv(adapter *Adapter, root string, acc Account, base []string) ([]string, error) {
	kvs, err := adapter.Env(root, acc)
	if err != nil {
		return nil, err
	}
	env := scrubEnv(base, adapter.Scrub)
	return mergeEnvKV(env, kvs), nil
}
