# Installing Codex from git

```bash
git clone https://github.com/openai/codex
cd codex/codex-rs
cargo b -r
sudo ln -fs $PWD/target/release/codex /usr/local/bin/codex-rs
```

## Usage

```bash
codex --profile devstral
```
