package remote_commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Fyve-Labs/tessa-daemon/internal/remote_commands/handler"
)

const (
	StartSSHCommand          = "start-ssh"
	EnableBeszelAgentCommand = "enable-beszel"
)

type CommandRequest struct {
	Command string      `json:"command"`
	Payload interface{} `json:"payload,omitempty"`
}

type Command struct {
	ID      string
	Payload interface{}
	manager *CommandManager
	handler handler.Handler
	ctx     context.Context    // Context for stopping the updater
	cancel  context.CancelFunc // Stops and removes command from updater
}

func (cmd *Command) getContext() (context.Context, context.CancelFunc) {
	if cmd.ctx == nil {
		cmd.ctx, cmd.cancel = context.WithCancel(context.Background())
	}

	return cmd.ctx, cmd.cancel
}

func (cmd *Command) Start() {
	if cmd.handler != nil {
		return
	}

	if cmd.ID == StartSSHCommand {
		h, err := handler.NewSSHServerHandler(cmd.Payload)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		cmd.handler = h
	}

	if cmd.handler == nil {
		return
	}

	go func() {
		cmd.manager.tunnelManager.ProxySSH("127.0.0.1", 2222)
		for {
			select {
			case <-cmd.ctx.Done():
				cmd.Stop()
				return
			}
		}
	}()

	if err := cmd.handler.Handle(cmd.ctx); err != nil {
		slog.Error(fmt.Sprintf("Failed to start command handler server: %v", err), slog.String("command", cmd.ID))
	}
}

func (cmd *Command) Stop() {
	if cmd.handler != nil {
		if err := cmd.handler.Stop(); err != nil {
			slog.Error(fmt.Sprintf("Failed to stop command hanler: %v", err), slog.String("command", cmd.ID))
		}
	}

	cmd.handler = nil
	cmd.manager.tunnelManager.UnProxy("127.0.0.1", 2222)
	cmd.cancel()
}
