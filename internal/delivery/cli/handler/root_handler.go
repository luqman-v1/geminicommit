package handler

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tfkhdyt/geminicommit/internal/service"
	"github.com/tfkhdyt/geminicommit/internal/usecase"
)

type RootHandler struct{}

func NewRootHandler() *RootHandler {
	return &RootHandler{}
}

func (r *RootHandler) RootCommand(
	ctx context.Context,
	stageAll *bool,
	autoSelect *bool,
	userContext *string,
	model *string,
	noConfirm *bool,
	quiet *bool,
	push *bool,
	dryRun *bool,
	showDiff *bool,
	maxLength *int,
	language *string,
	issue *string,
	noVerify *bool,
	provider *string,
	customBaseUrl *string,
) func(*cobra.Command, []string) {
	return func(_ *cobra.Command, _ []string) {
		if *quiet && !*noConfirm {
			*quiet = false
		}

		apiKey := viper.GetString("api.key")
		if apiKey == "" {
			fmt.Println(
				"Error: API key is still empty, run this command to set your API key",
			)
			fmt.Print("\n")
			color.New(color.Bold).Print("gmc config set ")
			color.New(color.Italic, color.Bold).Print("api.key YOUR_KEY\n\n")
			os.Exit(1)
		}

		aiService, err := service.NewAIService(ctx, *provider, apiKey, customBaseUrl)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		rootUsecase := usecase.NewRootUsecase(aiService)
		err = rootUsecase.RootCommand(ctx, stageAll, autoSelect, userContext, model, noConfirm, quiet, push, dryRun, showDiff, maxLength, language, issue, noVerify)
		cobra.CheckErr(err)
	}
}
