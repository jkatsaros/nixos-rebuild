package main

import (
  "os"
  "os/user"
  "time"
  "strconv"
  "errors"
  "path"

  "github.com/charmbracelet/log"
  "github.com/charmbracelet/huh"
)

type RebuildOptions struct {
  Flake string
  Rebuild bool
}

func main() {
  logger := log.NewWithOptions(os.Stderr, log.Options {
    ReportTimestamp: true,
    TimeFormat: time.DateTime,
  })

  var rebuildOptions RebuildOptions 

  accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))

  form := huh.NewForm(
    huh.NewGroup(
      huh.NewInput().
        Value(&rebuildOptions.Flake).
        Title("flake.nix location?").
        Placeholder(".dotfiles").
        Validate(func(s string) error {
          user, err := user.Current()
          if err != nil {
            logger.Fatal("Could not obtain current user information.")
          }

          path := path.Join(user.HomeDir, s, "flake.nix")
          if _, err := os.Open(path); errors.Is(err, os.ErrNotExist) {
            logger.Fatalf("Flake at %s does not exist", path)
          }
        
          return nil
        }).
        Description("This is a test."),
    ),

    huh.NewGroup(
      huh.NewConfirm().
        Title("Rebuild?").
        Value(&rebuildOptions.Rebuild),
    ),
  ).WithAccessible(accessible)

  err := form.Run()

  if err != nil {
    logger.Fatalf("Uh oh: %s", err)
  }

  logger.Debug("No changes detected.")

  if rebuildOptions.Rebuild {
    logger.Info("NixOS rebuild OK!")

    logger.Info("Home Manager rebuild OK!")

    logger.Info("Nix Flake update OK!")

    logger.Info("Success!")
  }
}
