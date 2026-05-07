# Release Process

## Releasing the TUI and Portal (`lingtai` repo)

### 1. Commit and push all changes

```bash
cd ~/Documents/GitHub/lingtai
git push origin main
```

### 2. Tag the release

```bash
git tag v0.X.Y
git push origin v0.X.Y
```

### 3. Create the GitHub release

```bash
gh release create v0.X.Y --title "v0.X.Y" --notes "release notes here..."
```

No binary assets needed — Homebrew builds from source, Linux users build locally.

### 4. Update the Homebrew tap

```bash
# Get the source tarball checksum
curl -sL "https://github.com/Lingtai-AI/lingtai/archive/refs/tags/v0.X.Y.tar.gz" | shasum -a 256

# Edit the formula
cd $(brew --repository)/Library/Taps/lingtai-ai/homebrew-lingtai
# In lingtai-tui.rb: update the url tag and sha256
git add lingtai-tui.rb
git commit -m "bump lingtai-tui to v0.X.Y"
git push
```

### 5. Verify

```bash
brew update && brew upgrade lingtai-ai/lingtai/lingtai-tui
lingtai-tui version  # should show v0.X.Y
```

### 6. Rebuild dev mode symlinks (if applicable)

After a release, your dev-mode symlinks still point at the old binary. Rebuild both:

```bash
cd ~/Documents/GitHub/lingtai/tui && make build
cd ~/Documents/GitHub/lingtai/portal && make build
```

## Releasing the kernel (`lingtai-kernel` repo)

The kernel is published to PyPI. After bumping `pyproject.toml` version:

```bash
cd ~/Documents/GitHub/lingtai-kernel
python -m build
twine upload dist/*
```

Or use the project's CI/CD pipeline if configured.

After publishing to PyPI, pull the kernel repo on your dev machine so the auto-upgrader doesn't clobber the editable install:

```bash
cd ~/Documents/GitHub/lingtai-kernel
git pull
```

## Installing without Homebrew

```bash
git clone https://github.com/Lingtai-AI/lingtai.git
cd lingtai/tui && make build
# Binary at tui/bin/lingtai-tui

cd ../portal && make build
# Binary at portal/bin/lingtai-portal
```

Requires Go toolchain and Node.js (for portal web frontend).

## Version scheme

- **TUI/portal:** Semantic versioning (`v0.X.Y`). Tags on the `lingtai` repo.
- **Kernel:** Semantic versioning. Published to PyPI. Version in `pyproject.toml`.
- **Migration versions:** Integer counter in `meta.json`. Separate for per-project (TUI+portal shared) and per-machine (global migrations).

## Post-release checklist

- [ ] Tag pushed to `lingtai` repo
- [ ] GitHub release created
- [ ] Homebrew tap updated
- [ ] `brew upgrade` verified
- [ ] Kernel published to PyPI (if kernel changed)
- [ ] Dev-mode symlinks rebuilt
- [ ] Kernel repo pulled on dev machine
