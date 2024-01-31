package extdebug

import (
	"github.com/rs/zerolog/log"
	"github.com/steadybit/steadybit-debug/config"
	"github.com/steadybit/steadybit-debug/debugrun"
	"github.com/steadybit/steadybit-debug/output"
	"os"
)

func RunSteadybitDebug(workingDir string) string{
	cfg := config.GetConfig()

	cfg.OutputPath = workingDir

	output.AddOutputDirectory(&cfg)

	output.AddJsonOutput(output.AddJsonOutputOptions{
		Config:     &cfg,
		Content:    cfg,
		OutputPath: []string{"debugging_config.yaml"},
	})
	debugrun.GatherInformation(&cfg)
	zipResult := output.ZipOutputDirectory(&cfg)
	log.Info().Msgf("Debugging output collected at: %s", zipResult)

	err := os.RemoveAll(cfg.OutputPath)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to remove output directory '%s' after completion", cfg.OutputPath)
	}
	return zipResult
}
