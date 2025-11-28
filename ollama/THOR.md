# Setting up in Jetson Thor

See the instructions in README.md

1. Edit '/etc/nv_tegra_release' file to contain R36 to fix ollama installer (it will download jetpack6). you can use jetpack5 if you do R35 in there.
2. install ollama with curl -fsSL https://ollama.com/install.sh | sh
3. Edit the systemd ollama service to specify the env vars
4. Note that LD_LIBRARY_PATH is mandatory to fix startup segfault

# Local VibeCoding

Decent models for local coding are qwen3-coder and devstral
See the codex config and use the custom prompt.md from the codex directory from this repo
