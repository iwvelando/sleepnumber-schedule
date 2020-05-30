package main

import (
	"flag"
	"fmt"
	"github.com/iwvelando/SleepIQ"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"os"
	"time"
)

// Configuration represents a YAML-formatted config file
type Configuration struct {
	SleepIQUsername       string
	SleepIQPassword       string
	BedStatusPollInterval time.Duration
	BedStatusPollMax      time.Duration
}

func LoadConfiguration(configPath string) (*Configuration, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()
	viper.SetConfigType("yml")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config file %s, %s", configPath, err)
	}

	var configuration Configuration
	err = viper.Unmarshal(&configuration)
	if err != nil {
		return nil, fmt.Errorf("unable to decode config into struct, %s", err)
	}

	return &configuration, nil
}

func main() {
	// Initialize the structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Println("{\"op\": \"main\", \"level\": \"fatal\", \"msg\": \"failed to initiate logger\"}")
		os.Exit(1)
	}
	defer logger.Sync()

	// Read runtime parameters from the CLI
	bedName := flag.String(
		"bed-name",
		"",
		"name of the target bed (Account Settings -> My Sleep Number Beds -> X/Y Bed Online -> <Bed Name>)",
	)

	side := flag.String(
		"side",
		"",
		"which side of the bed to be altered (Left or Right)",
	)

	position := flag.Int(
		"position",
		0,
		"which position to target based on 1=Favorite, 2=Read, 3=WatchTV, 4=Flat, 5=ZeroG, 6=Snore",
	)

	configLocation := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	// Load the config file based on path provided via CLI or the default
	config, err := LoadConfiguration(*configLocation)
	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to load configuration at %s", *configLocation),
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	if *bedName == "" {
		logger.Fatal("must specify bed-name parameter",
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	if *side == "" {
		logger.Fatal("must specify side parameter",
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	if *position == 0 {
		logger.Fatal("must specify position parameter",
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	// Initialize the SleepIQ client and login
	siq := sleepiq.New()

	_, err = siq.Login(config.SleepIQUsername, config.SleepIQPassword)
	if err != nil {
		logger.Fatal("failed to log into SleepIQ account",
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	// Get information about all the beds
	beds, err := siq.Beds()
	if err != nil {
		logger.Fatal("failed to query beds",
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	// Identify the target bed
	index := -1
	var targetBed sleepiq.Bed
	for i, bed := range beds.Beds {
		if bed.Name == *bedName {
			index = i
			targetBed = bed
			logger.Debug(fmt.Sprintf("identified bed %s at index %d", *bedName, i),
				zap.String("op", "main"),
			)
			break
		}
	}

	if index == -1 {
		logger.Fatal(fmt.Sprintf("failed to identify target bed %s", *bedName),
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	// Issue command to set bed to target position
	logger.Info("setting bed to target position",
		zap.String("op", "main"),
	)
	bedStatus, err := siq.ControlBedPosition(targetBed.BedID, *side, *position)
	if err != nil {
		logger.Fatal("failed to set bed to target position",
			zap.String("op", "main"),
			zap.Error(err),
		)
	}

	// Do not exit until the movement has stopped
	t0 := time.Now()
	for bedStatus.IsMoving {
		bedStatus, err = siq.BedFoundationStatus(targetBed.BedID)
		if err != nil {
			logger.Fatal("failed to query bed status to check whether movement has ended",
				zap.String("op", "main"),
				zap.Error(err),
			)
		}
		time.Sleep(config.BedStatusPollInterval * time.Second)
		if time.Since(t0) >= config.BedStatusPollMax*time.Second {
			logger.Warn("reached maximum bed status polling time, quitting",
				zap.String("op", "main"),
			)
			os.Exit(1)
		}
	}
	logger.Info("movement has stopped",
		zap.String("op", "main"),
	)

	os.Exit(0)

}
