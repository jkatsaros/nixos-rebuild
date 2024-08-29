#!/usr/bin/env bash

# Credit: 0atman (GitHub)

if command -v gum >/dev/null 2>&1 ; then
  gum log --structured --level debug --time datetime "Gum found!"
else
  echo "Gum not found. Running 'nix-env --install gum'..."
  nix-env --install gum
  gum -v
fi

# Notify when no changes are detected.
if git diff --quiet '*.nix'; then
  gum log --structured --level debug --time datetime "No changes detected."
fi

# Show your changes.
git diff -U0 '*.nix'

if gum confirm "Rebuild?"; then
  # 0. Stage changes for rebuilding.
  # NixOS won't rebuild new modules if they're not staged.
  git add .

  # 1. NixOS
  gum spin --spinner points --title "NixOS rebuilding..." --show-output --show-error -- sudo nixos-rebuild switch --flake .
  gum log --structured --level info --time datetime "NixOS rebuild OK!"

  # 2. Home-Manager
  gum spin --spinner points --title "Home Manager rebuilding..." --show-output --show-error -- home-manager switch --flake .
  gum log --structured --level info --time datetime "Home Manager rebuild OK!"

  # 3. Flake
  gum spin --spinner points --title "Nix Flake updating..." --show-output --show-error -- nix flake update
  gum log --structured --level info --time datetime "Nix Flake update OK!"

  # Get current generation's metadata.
  current=$(nixos-rebuild list-generations | grep current)

  # Commit all changes with the generation's metadata.
  gum confirm "Commit changes?" && git commit -am "$current"
fi

gum log --structured --level info --time datetime "Success!"

