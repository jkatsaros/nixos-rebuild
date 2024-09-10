package main

import (
  "os"
  "os/user"
  "os/exec"
  "time"
  "strconv"
  "errors"
  "path"
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
  }
  RebuildSettings struct {
    ConfigurationNixPath string `yaml:"ConfigurationNixPath"`
    UsingHomeManager bool `yaml:"UsingHomeManager"`
    HomeNixPath string `yaml:"HomeNixPath"`
    UsingFlakes bool `yaml:"UsingFlakes"`
    FlakeNixPath string `yaml:"FlakeNixPath"`
  }
}

func loadConfiguration() (Configuration, error) {
  var configuration Configuration
  
  file, err := os.ReadFile("configuration.yaml")
  if err != nil {
    log.Error("Could not read configuration file.")
    return nil, err
  }

  err = yaml.Unmarshal(file, &configuration)
  if err != nil {
    log.Error("Could not load configuration file.")
    return nil, err
  }
  
  return configuration, nil
}

func (configuration *Configuration, logger Logger) saveConfiguration() error {
  file, err := yaml.Marshal(&configuration)
  if err != nil {
    logger.Error("Could not to load configuration.")
    return err
  }

  f, err := os.Open("configuration.yaml")
  if err != nil {
    logger.Error("Could not to read configuration file.")
    return err
  }
  defer f.Close()

  _, err = io.WriteString(f, string(file))
  if err != nil {
    logger.Error("Could not to write configuration file.")
    return err
  }
}

func (s string, name string) validatePath() error {
  var p string
  
  user, err := user.Current()
  if err != nil {
    logger.Error("Could not obtain current user information.")
    return err
  }
  
  if s == "~" {
    p = path.Join(user.HomeDir, name)
  } else if strings.HasPrefix(s, "~/")
    p = path.Join(user.HomeDir, s, name)
  if strings.HasPrefix(s, "/") {
    p = path.Join(s, name)
  }
  
  if _, err := os.Open(p); errors.Is(err, os.ErrNotExist) {
    logger.Errorf("%s at %s does not exist.", name, p)
    return err
  }

  return nil
}

func (title string, command Cmd, accessible bool, logger Logger) executeNixCommand() error {
  err := spinner.
    New().
    Title(title).
    Action(command.Run()).
    Accessible(accessible).
    Run()
    
  if err != nil {
    logger.Error("Could not execute command.")
    return err
  }
  
  return nil
}

func (command Cmd, logger Logger) executeGitCommand() error {
  stdout, err := command.StdoutPipe()
  if err != nil {
    logger.Error(err)
    return err
  }
  
  if err := command.Start(); err != nil {
    logger.Error(err)
    return err
  }
  
  logger.Debug(stdout)
  
  if err := command.Wait(); err != nil {
    logger.Error(err)
    return err
  }
  
  return nil
}

func main() {
  configuration, err := loadConfiguration()
  if err != nil {
    log.Fatal(err)
  }
  
  logger := log.NewWithOptions(
    os.Stderr,
    log.Options {
      TimeFormat: configuration.LoggingSettings.TimeFormat,
      Prefix: configuration.LoggingSettings.Prefix,
      ReportTimestamp: configuration.LoggingSettings.ReportTimestamp,
      ReportCaller: configuration.LoggingSettings.ReportCaller
    }
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
        Validate(if err := validatePath(s, "configuration.nix"); logger.Fatal(err)),
    ).
    WithHide(if len(strings.TrimSpace(flakeNixPath)) != 0),
    
    huh.NewGroup(
      huh.NewInput().
        Value(&homeNixPath).
        Title("Enter the path to 'home.nix':")
        Placeholder(".dotfiles").
        Validate(if err := validatePath(s, "home.nix"); logger.Fatal(err)),
    ).
    WithHide(if usingHomeManager && len(strings.TrimSpace(homeNixPath)) != 0),
    
    huh.NewGroup(
      huh.NewInput().
        Value(&flakeNixPath).
        Title("Enter the path to 'flake.nix':")
        Placeholder(".dotfiles").
        Validate(if err := validatePath(s, "flake.nix"); logger.Fatal(err)),
    ).
    WithHide(if usingFlakes && len(strings.TrimSpace(flakeNixPath)) != 0),

    huh.NewGroup(
      huh.NewConfirm().
        Title("Rebuild?").
        Value(&shouldRebuild),
    ),
  ).
  WithAccessible(accessible)

  err := form.Run()
  if err != nil {
    logger.Fatal(err)
  }
  
  err := saveConfiguration(configuration, logger)
  if err != nil {
    logger.Fatal(err)
  }

  gitDiffQuietCmd := exec.Command("git", "diff", "--quiet", "'*.nix'")
  err := executeGitCommand(gitDiffQuietCmd, logger)
  if err != nil {
    logger.Fatal("Could not .")
  }
  logger.Debug("No changes detected.")
  
  gitDiffCmd := exec.Command("git", "diff", "-U0", "'*.nix'")
  err := executeGitCommand(gitDiffCmd, logger)
  if err != nil {
    logger.Fatal("Could not .")
  }

  if shouldRebuild {
    gitAddAllCmd := exec.Command("git", "add", ".")
    err := executeGitCommand(gitAddAllCmd, logger)
    if err != nil {
      logger.Fatal("Could not .")
    }
    
    var nixosRebuildCmd Cmd
    if usingFlakes {
      nixosRebuildCmd := exec.Command("sudo", "nixos-rebuild", "switch", "--flake", flakeNixPath)
    } else {
      nixosRebuildCmd := exec.Command("sudo", "nixos-rebuild", "switch", configurationNixPath)
    }
    if err := executeNixCommand("NixOS rebuilding...", nixosRebuildCmd, accessible, logger); err != nil {
      logger.Fatal("Could not rebuild NixOS.")
    }
    logger.Info("NixOS rebuild OK!")

    if usingHomeManager {
      var homemanagerRebuildCmd Cmd
      if usingFlakes {
      homemanagerRebuildCmd := exec.Command("home-manager", "switch", "--flake", flakeNixPath)
      } else {
        homemanagerRebuildCmd := exec.Command("home-manager", "switch", homeNixPath)
      }
      if err := executeNixCommand("Home Manager rebuilding...", homemanagerRebuildCmd, accessible, logger); err != nil {
        logger.Fatal("Could not rebuild Home Manager.")
      }
      logger.Info("Home Manager rebuild OK!")
    }

    if usingFlakes {
      flakeRebuildCmd := exec.Command("nix", "flake", "update")
      if err := executeNixCommand("Flake updating...", flakeRebuildCmd, accessible, logger); err != nil {
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
    )
    .WithAccessible(accessible)
    
    err := form.Run()
    if err != nil {
      logger.Fatal(err)
    }
    
    if commitChanges {
      getCurrentNixGenerationCmd := exec.Command("nixos-rebuild", "list-generations", "|", "grep", "current")
      &currentGeneration, err := getCurrentNixGenerationCmd.StdoutPipe()
      if err != nil {
        logger.Fatal(err)
      }
  
      if err := getCurrentNixGenerationCmd.Start(); err != nil {
        logger.Fatal(err)
      }
  
      if err := getCurrentNixGenerationCmd.Wait(); err != nil {
        logger.Fatal(err)
      }
      
      gitCommitCmd := exec.Command("git", "commit", "-am", currentGeneration)
      err := executeGitCommand(gitCommitCmd, logger)
      if err != nil {
        logger.Fatal("Could not commit changes.")
      }
    }

    logger.Info("Success!")
  }
}
