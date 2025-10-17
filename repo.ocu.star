ocuroot("0.3.0")

repo_alias = "github.com/ocuroot/ocuroot"

state_store = store.git("ssh://git@github.com/ocuroot/ocuroot-state.git")
intent_store = store.git("ssh://git@github.com/ocuroot/ocuroot-intent.git")

release_ignore = [
    "tests/**",
    "examples/**",
]