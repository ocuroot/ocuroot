ocuroot("0.3.0")

# Define example environments for previews
# Ordinarily this would be configured in state and loaded using `environments()`
envs = [
    environment("staging", {"type": "staging"}),
    environment("production", {"type": "prod"}),
    environment("production2", {"type": "prod"}),
]
