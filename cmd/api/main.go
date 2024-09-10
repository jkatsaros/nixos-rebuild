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
    log.Error("Failed to read configuration file.")
    return nil, err
  }

  err = yaml.Unmarshal(file, &configuration)
  if err != nil {
    log.Error("Failed to load configuration file.")
    return nil, err
  }
  
  return configuration, nil
}

func (configuration Configuration, logger Logger) saveConfiguration() error {
  file, err := yaml.Marshal(&configuration)
  if err != nil {
    logger.Error("Failed to load configuration.")
    return err
  }

  fmt.Println(string(file))

  f, err := os.Open("configuration.yaml")
  if err != nil {
    logger.Error("Failed to read configuration file.")
    return err
  }
  defer f.Close()

  _, err = io.WriteString(f, string(file))
  if err != nil {
    logger.Error("Failed to write configuration file.")
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
  )

  accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))

  form := huh.NewForm(
    huh.NewGroup(
      huh.NewInput().
        Value(&flakeNixPath).
        Title("Enter the path to configuration.nix:").
        Placeholder(".dotfiles").
        Validate(if err := validatePath(s, "configuration.nix"); logger.Fatal(err)).
        Description("This is a test."),
    ).
    WithHide(if len(strings.TrimSpace(flakeNixPath)) != 0),
    
    huh.NewGroup(
      huh.NewInput().
        Value(&homeNixPath).
        Title("Enter the path to home.nix:")
        Placeholder(".dotfiles").
        Validate(if err := validatePath(s, "home.nix"); logger.Fatal(err)).
        Description("This is a test.")
    ).
    WithHide(if usingHomeManager && len(strings.TrimSpace(homeNixPath)) != 0),
    
    huh.NewGroup(
      huh.NewInput().
        Value(&flakeNixPath).
        Title("Enter the path to flake.nix:")
        Placeholder(".dotfiles").
        Validate(if err := validatePath(s, "flake.nix"); logger.Fatal(err)).
        Description("This is a test.")
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

  logger.Debug("No changes detected.")

  if rebuildOptions.Rebuild {
    var nixosRebuildCmd Cmd
    if usingFlakes {
      nixosRebuildCmd := exec.Command(fmt.Printf("sudo nixos-rebuild switch --flake %s", flakeNixPath))
    } else {
      nixosRebuildCmd := exec.Command(fmt.Printf("sudo nixos-rebuild switch %s", configurationNixPath))
    }
    if err := spinner.
      New().
      Title("Rebuilding NixOS...").
      Action(nixosRebuildCmd.Run()).
      Accessible(accessible).
      Run(); err != nil {
      logger.Fatal("NixOS rebuild failed!")
    }
    logger.Info("NixOS rebuild OK!")

    if usingHomeManager {
      var homemanagerRebuildCmd Cmd
      if usingFlakes {
      homemanagerRebuildCmd := exec.Command(fmt.Printf("home-manager switch --flake %s", flakeNixPath))
      } else {
        homemanagerRebuildCmd := exec.Command(fmt.Printf("home-manager switch %s", homeNixPath))
      }
      if err := spinner.
        New().
        Title("Rebuilding Home Manager...").
        Action(homemanagerRebuildCmd.Run()).
        Accessible(accessible).
        Run(); err != nil {
        logger.Fatal("Home Manager rebuild failed!")
      }
      logger.Info("Home Manager rebuild OK!")
    }

    if usingFlakes {
      flakeRebuildCmd := exec.Command(fmt.Printf("home-manager switch --flake %s", flakeNixPath))
      if err := spinner.
        New().
        Title("Updating Flake...").
        Action(flakeRebuildCmd.Run()).
        Accessible(accessible).
        Run(); err != nil {
        logger.Fatal("Nix Flake update failed!")
      }
      logger.Info("Nix Flake update OK!")
    }

    logger.Info("Success!")
  }
}
