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
		cmd.startSSHServer()
	}

	for {
		select {
		case <-cmd.ctx.Done():
			cmd.Stop()
			return
		}
	}
}

func (cmd *Command) startSSHServer() {
	if cmd.handler != nil {
		return
	}

	h, err := handler.NewSSHServerHandler(cmd.Payload)
	if err != nil {
		slog.Error(fmt.Sprintf("new SSH server: %v", err), slog.String("command", cmd.ID))
		return
	}
	cmd.handler = h

	go func() {
		if err := cmd.handler.Handle(cmd.ctx); err != nil {
			slog.Error(fmt.Sprintf("start ssh server: %v", err), slog.String("command", cmd.ID))
		}
	}()

	cmd.manager.tunnelManager.ProxySSH("127.0.0.1", h.ListenPort())
}

func (cmd *Command) Stop() {
	if cmd.handler != nil {
		if err := cmd.handler.Stop(); err != nil {
			slog.Error(fmt.Sprintf("Failed to stop command hanler: %v", err), slog.String("command", cmd.ID))
		}

		if h, ok := cmd.handler.(*handler.SSHServerHandler); ok {
			cmd.manager.tunnelManager.UnProxy("127.0.0.1", h.ListenPort())
		}
	}

	cmd.handler = nil
	cmd.cancel()
}
