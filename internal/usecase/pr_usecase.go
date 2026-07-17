/*
Copyright © 2025 Christina Sørensen <ces@fem.gg>
*/
package usecase

import (
	"context"

	"github.com/fatih/color"

	"github.com/tfkhdyt/geminicommit/internal/service"
)

type PRUsecase struct {
	gitService         *service.GitService
	aiService          service.AIService
	interactionService *service.InteractionService
}

func NewPRUsecase(aiService service.AIService) *PRUsecase {
	return &PRUsecase{
		gitService:         service.NewGitService(),
		aiService:          aiService,
		interactionService: service.NewInteractionService(),
	}
}

func (p *PRUsecase) PRCommand(
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
) error {
	if err := p.gitService.VerifyGitInstallation(); err != nil {
		return err
	}

	if err := p.gitService.VerifyGitRepository(); err != nil {
		return err
	}

	opts := &service.CommitOptions{
		Model:       model,
		NoConfirm:   noConfirm,
		Quiet:       quiet,
		DryRun:      dryRun,
		ShowDiff:    showDiff,
		MaxLength:   maxLength,
		Language:    language,
		UserContext: userContext,
	}

	data, err := p.gitService.GetDiff()
	if err != nil {
		return err
	}

	if *opts.ShowDiff && !*opts.Quiet {
		p.interactionService.DisplayDiff(data.Diff)
	}

	for {
		message, err := p.aiService.GenerateCommitMessage(ctx, data, opts)
		if err != nil {
			return err
		}

		selectedAction, finalMessage, err := p.interactionService.HandleUserAction(
			message,
			opts,
		)
		if err != nil {
			return err
		}

		switch selectedAction {
		case service.ActionConfirm:
			if err := p.gitService.CreatePullRequest(
				finalMessage,
				opts.Quiet,
				opts.DryRun,
				draft,
			); err != nil {
				return err
			}
			return nil
		case service.ActionRegenerate:
			continue
		case service.ActionEditContext:
			continue
		case service.ActionCancel:
			color.New(color.FgRed).Println("Pull request cancelled")
			return nil
		}
	}
}
