/*
Copyright © 2025 Christina Sørensen <ces@fem.gg>
*/
package handler

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/luqman-v1/geminicommit/internal/service"
	"github.com/luqman-v1/geminicommit/internal/usecase"
)

type PRHandler struct{}

func NewPRHandler() *PRHandler {
	return &PRHandler{}
}

func (p *PRHandler) PRCommand(
	ctx context.Context,
	model *string,
	noConfirm *bool,
	quiet *bool,
	dryRun *bool,
	showDiff *bool,
	maxLength *int,
	language *string,
	userContext *string,
	draft *bool,
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

		prUsecase := usecase.NewPRUsecase(aiService)
		err = prUsecase.PRCommand(
			ctx,
			model,
			noConfirm,
			quiet,
			dryRun,
			showDiff,
			maxLength,
			language,
			userContext,
			draft,
		)
		cobra.CheckErr(err)
	}
}
