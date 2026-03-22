---
name: devenv
description: Manage the dev container Dockerfile at .herm/Dockerfile
---

Manage the single dev container Dockerfile at .herm/Dockerfile. The built image replaces the running container and persists across sessions. Use this to install languages, tools, compilers, and system dependencies permanently. Always read before writing.

ONE environment per project. There is exactly one Dockerfile. When adding new tools, extend it — never create a parallel one.

This is the ONLY way to install tools persistently. Ad-hoc installs via bash (apt-get, apk add, pip install, npm install -g) are ephemeral and lost on container restart.

**All Dockerfiles MUST use `FROM aduermael/herm:__HERM_VERSION__` as the base image.** The `__HERM_VERSION__` placeholder is resolved automatically at build time. This image includes git, ripgrep, tree, python3, and the herm file tools. Do NOT use other base images or hardcode version tags — builds will be rejected.

**Mandatory workflow: read -> write -> build. Never skip read.**
- read: always do this first. See what's already installed.
- write: provide the COMPLETE Dockerfile starting with `FROM aduermael/herm:__HERM_VERSION__`. Keep everything already there, add what's new.
- build: apply the new image. The running container is hot-swapped.

Build proactively. Before running code that requires tools not in the current image (__CONTAINER_IMAGE__), use devenv first. Don't wait for errors.

**Dockerfile rules that prevent build failures:**
- Always extend the herm base image. Add languages and tools on top of it via apt-get.
- Look at how official Docker images (golang, node, python) install their runtimes — replicate that approach. Download official release tarballs and extract them, or use distro packages.
- Never use curl-pipe-to-bash third-party setup scripts (NodeSource setup_lts.x, rustup.sh, etc). They are fragile and break in non-interactive build environments.
- Combine related RUN steps: apt-get update && apt-get install -y ... && rm -rf /var/lib/apt/lists/*. Never split update and install across layers.
- Pin specific versions for reproducibility. Set WORKDIR /workspace.

If a build fails: read the error carefully, identify the specific failing RUN step, fix only that, then build again.
