package main

import (
  "os"
  "os/exec"
  "strconv"
  "errors"
  "strings"
  "io"
  
  "gopkg.in/yaml.v3"

  "github.com/charmbracelet/log"
  "github.com/charmbracelet/huh"
  "github.com/charmbracelet/huh/spinner"
)

type Configuration struct {
  LoggingSettings struct {
    TimeFormat string `yaml:"TimeFormat"`
    Prefix string `yaml:"Prefix"`
    ReportTimestamp bool `yaml:"ReportTimestamp"`
    ReportCaller bool `yaml:"ReportCaller"`
  } `yaml:"LoggingSettings"`
  RebuildSettings struct {
    ConfigurationNixPath string `yaml:"ConfigurationNixPath"`
    UsingHomeManager bool `yaml:"UsingHomeManager"`
    HomeNixPath string `yaml:"HomeNixPath"`
    UsingFlakes bool `yaml:"UsingFlakes"`
    FlakeNixPath string `yaml:"FlakeNixPath"`
  } `yaml:"RebuildSettings"`
}

func main() {
  var configuration Configuration
  
  file, err := os.ReadFile("configuration.yaml")
  if err != nil {
    log.Fatal("Could not read configuration file.")
  }

  err = yaml.Unmarshal(file, &configuration)
  if err != nil {
    log.Error(err)
    log.Fatal("Could not load configuration file.")
  }
  
  logger := log.NewWithOptions(
    os.Stderr,
    log.Options {
      TimeFormat: configuration.LoggingSettings.TimeFormat,
      Prefix: configuration.LoggingSettings.Prefix,
      ReportTimestamp: configuration.LoggingSettings.ReportTimestamp,
      ReportCaller: configuration.LoggingSettings.ReportCaller,
    },
  )
  
  var (
    configurationNixPath = configuration.RebuildSettings.ConfigurationNixPath
    usingHomeManager = configuration.RebuildSettings.UsingHomeManager
    homeNixPath = configuration.RebuildSettings.HomeNixPath
    usingFlakes = configuration.RebuildSettings.UsingFlakes
    flakeNixPath = configuration.RebuildSettings.FlakeNixPath
    shouldRebuild bool
    shouldCommit bool
    currentGeneration string
  )

  accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))

  form := huh.NewForm(
    huh.NewGroup(
      huh.NewInput().
        Value(&flakeNixPath).
        Title("Enter the path to 'configuration.nix':").
        Placeholder(".dotfiles").
        Validate(func(s string) error {
          if _, err := os.Open(s); errors.Is(err, os.ErrNotExist) {
            logger.Errorf("File does not exist at: %s", s)
          }

          return nil
        }),
    ).
    WithHide(len(strings.TrimSpace(flakeNixPath)) != 0),
    
    huh.NewGroup(
      huh.NewInput().
        Value(&homeNixPath).
        Title("Enter the path to 'home.nix':").
        Placeholder(".dotfiles").
        Validate(func(s string) error {
          if _, err := os.Open(s); errors.Is(err, os.ErrNotExist) {
            logger.Errorf("File does not exist at: %s", s)
          }

          return nil
        }),
    ).
    WithHide(usingHomeManager && len(strings.TrimSpace(homeNixPath)) != 0),
    
    huh.NewGroup(
      huh.NewInput().
        Value(&flakeNixPath).
        Title("Enter the path to 'flake.nix':").
        Placeholder(".dotfiles").
        Validate(func(s string) error {
          if _, err := os.Open(s); errors.Is(err, os.ErrNotExist) {
            logger.Errorf("File does not exist at: %s", s)
          }

          return nil
        }),
    ).
    WithHide(usingFlakes && len(strings.TrimSpace(flakeNixPath)) != 0),

    huh.NewGroup(
      huh.NewConfirm().
        Title("Rebuild?").
        Value(&shouldRebuild),
    ),
  ).
  WithAccessible(accessible)

  err = form.Run()
  if err != nil {
    logger.Fatal(err)
  }
  
  //file, err = yaml.Marshal(&configuration)
  //if err != nil {
  //  logger.Fatal("Could not to load configuration.")
  //}

  //f, err := os.Open("configuration.yaml")
  //if err != nil {
  //  logger.Fatal("Could not to read configuration file.")
  //}
  //defer f.Close()

  //_, err = io.WriteString(f, string(file))
  //if err != nil {
  //  logger.Fatal("Could not to write configuration file.")
  //}

  gitDiffQuietCmd := exec.Command("git", "diff", "--quiet", "'*.nix'")
  stdout, err := gitDiffQuietCmd.StdoutPipe()
  if err != nil {
    logger.Fatal(err)
  }
  
  if err := gitDiffQuietCmd.Start(); err != nil {
    logger.Fatal(err)
  }
  
  log.Debug(stdout)
  
  if err := gitDiffQuietCmd.Wait(); err != nil {
    log.Fatal(err)
  }

  logger.Debug("No changes detected.")
  
  gitDiffCmd := exec.Command("git", "diff", "-U0", "'*.nix'")
  stdout, err = gitDiffCmd.StdoutPipe()
  if err != nil {
    logger.Fatal(err)
  }

  if err := gitDiffCmd.Start(); err != nil {
    logger.Fatal(err)
  }

  log.Debug(stdout)

  if err := gitDiffCmd.Wait(); err != nil {
    log.Fatal(err)
  }

  if shouldRebuild {
    gitAddAllCmd := exec.Command("git", "add", ".")
    stdout, err = gitAddAllCmd.StdoutPipe()
    if err != nil {
      logger.Fatal(err)
    }

    if err := gitAddAllCmd.Start(); err != nil {
      logger.Fatal(err)
    }

    log.Debug(stdout)

    if err := gitAddAllCmd.Wait(); err != nil {
      log.Fatal(err)
    }

    err = spinner.
      New().
      Title("NixOS rebuilding...").
      Action(func() {
        if usingFlakes {
          err := exec.Command("sudo", "nixos-rebuild", "switch", "--flake", flakeNixPath).Run()
          if err != nil {
            logger.Fatal(err)
          }
        } else {
          err := exec.Command("sudo", "nixos-rebuild", "switch", configurationNixPath).Run()
          if err != nil {
            logger.Fatal(err)
          }
        }
      }).
      Accessible(accessible).
      Run()
    
    if err != nil {
      logger.Fatal("Could not rebuild NixOS.")
    }
    logger.Info("NixOS rebuild OK!")

    if usingHomeManager {
      err = spinner.
        New().
        Title("Home Manager rebuilding...").
        Action(func() {
          if usingFlakes {
            err := exec.Command("home-manager", "switch", "--flake", flakeNixPath).Run()
            if err != nil {
              logger.Fatal(err)
            }
          } else {
            err := exec.Command("home-manager", "switch", homeNixPath).Run()
            if err != nil {
              logger.Fatal(err)
            }
          }
        }).
        Accessible(accessible).
        Run()
    
      if err != nil {
        logger.Fatal("Could not rebuild Home Manager.")
      }
      logger.Info("Home Manager rebuild OK!")
    }

    if usingFlakes {
      err = spinner.
        New().
        Title("Flake updating...").
        Action(func() {
          err := exec.Command("nix", "flake", "update").Run()
          if err != nil {
            logger.Fatal(err)
          }
        }).
        Accessible(accessible).
        Run()
    
      if err != nil {
        logger.Fatal("Could not update Flake.")
      }
      logger.Info("Flake update OK!")
    }
    
    form := huh.NewForm(
      huh.NewGroup(
        huh.NewConfirm().
          Title("Commit Changes?").
          Value(&shouldCommit),
      ),
    ).
    WithAccessible(accessible)
    
    err = form.Run()
    if err != nil {
      logger.Fatal(err)
    }
    
    if shouldCommit {
      getCurrentNixGenerationCmd := exec.Command("nixos-rebuild", "list-generations", "|", "grep", "current")
      stdout, err = getCurrentNixGenerationCmd.StdoutPipe()
      if err != nil {
        logger.Fatal(err)
      }
  
      if err := getCurrentNixGenerationCmd.Start(); err != nil {
        logger.Fatal(err)
      }

      logger.Debug(stdout)
  
      if err := getCurrentNixGenerationCmd.Wait(); err != nil {
        logger.Fatal(err)
      }

      bytes, err := io.ReadAll(stdout)
      if err != nil {
        logger.Fatal(err)
      }
      currentGeneration = string(bytes)
      
      gitCommitCmd := exec.Command("git", "commit", "-am", string(currentGeneration))
      stdout, err = gitCommitCmd.StdoutPipe()
      if err != nil {
        logger.Fatal(err)
      }
  
      if err := gitCommitCmd.Start(); err != nil {
        logger.Fatal(err)
      }

      logger.Debug(stdout)
  
      if err := gitCommitCmd.Wait(); err != nil {
        logger.Fatal(err)
      }
    }

    logger.Info("Success!")
  }
}
